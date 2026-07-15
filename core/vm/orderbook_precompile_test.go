package vm

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/params"
)

func u256Pad32(v *uint256.Int) []byte {
	if v == nil {
		v = uint256.NewInt(0)
	}
	return ethcommon.LeftPadBytes(v.ToBig().Bytes(), 32)
}

func packPlace(side byte, token crypto.Address, price, base *uint256.Int) []byte {
	out := make([]byte, 0, 86)
	out = append(out, ObMethodPlace, side)
	out = append(out, token[:]...)
	out = append(out, u256Pad32(price)...)
	out = append(out, u256Pad32(base)...)
	return out
}

func setupOrderbook(t *testing.T) (
	exec *Executor,
	statedb *state.StateDB,
	maker, taker, token, obAddr crypto.Address,
	parsed abi.ABI,
) {
	t.Helper()
	mdb := db.OpenTest(t)
	statedb = state.New(mdb)
	maker = crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	taker = crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	fund := uint256.MustFromBig(new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil))
	statedb.SetBalance(maker, fund)
	statedb.SetBalance(taker, new(uint256.Int).Set(fund))

	exec = NewExecutor(statedb, BlockContext{
		Number: 1, Time: 1, GasLimit: 30_000_000, BaseFee: big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(int64(params.PublicTestnetChainID)),
	})
	exec.EnableDewPrecompiles(true)
	exec.EnableNativeSwap(true)

	var err error
	parsed, err = abi.JSON(strings.NewReader(TokenABI))
	if err != nil {
		t.Fatal(err)
	}
	bin, err := hex.DecodeString(TokenCreationBytecode)
	if err != nil {
		t.Fatal(err)
	}
	supply := new(big.Int).Mul(big.NewInt(1_000_000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	ctor, err := parsed.Pack("", supply)
	if err != nil {
		t.Fatal(err)
	}
	res, err := exec.ApplyMessage(Message{
		From: maker, To: nil, Value: uint256.NewInt(0),
		GasLimit: 3_000_000, GasPrice: big.NewInt(0),
		Data: append(append([]byte{}, bin...), ctor...),
	})
	if err != nil || res.Failed || res.ContractAddress == nil {
		t.Fatalf("deploy token: err=%v failed=%v %v", err, res.Failed, res.Err)
	}
	token = *res.ContractAddress
	copy(obAddr[:], NativeSwapPrecompile[:])
	return exec, statedb, maker, taker, token, obAddr, parsed
}

func tokenBalanceOf(t *testing.T, exec *Executor, parsed abi.ABI, token, who crypto.Address) *uint256.Int {
	t.Helper()
	data, err := parsed.Pack("balanceOf", ethcommon.BytesToAddress(who[:]))
	if err != nil {
		t.Fatal(err)
	}
	res, err := exec.ApplyMessage(Message{
		From: who, To: &token, Value: uint256.NewInt(0),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Data: data,
	})
	if err != nil || res.Failed {
		t.Fatalf("balanceOf: %v %v", err, res.Err)
	}
	return new(uint256.Int).SetBytes(res.ReturnData)
}

func approveToken(t *testing.T, exec *Executor, parsed abi.ABI, owner, token, spender crypto.Address, amount *uint256.Int) {
	t.Helper()
	data, err := parsed.Pack("approve", ethcommon.BytesToAddress(spender[:]), amount.ToBig())
	if err != nil {
		t.Fatal(err)
	}
	res, err := exec.ApplyMessage(Message{
		From: owner, To: &token, Value: uint256.NewInt(0),
		GasLimit: 100_000, GasPrice: big.NewInt(0), Data: data,
	})
	if err != nil || res.Failed {
		t.Fatalf("approve: %v %v", err, res.Err)
	}
}

func TestOrderbook_PlaceBuyCancel_Refund(t *testing.T) {
	exec, statedb, maker, _, token, ob, _ := setupOrderbook(t)
	price := uint256.NewInt(2e18)
	base := uint256.NewInt(1e18)
	lock := uint256.NewInt(2e18)
	before := new(uint256.Int).Set(statedb.GetBalance(maker))
	res, err := exec.ApplyMessage(Message{
		From: maker, To: &ob, Value: lock,
		GasLimit: 200_000, GasPrice: big.NewInt(0),
		Data: packPlace(0, token, price, base),
	})
	if err != nil || res.Failed {
		t.Fatalf("place buy: %v %v", err, res.Err)
	}
	orderID := new(uint256.Int).SetBytes(res.ReturnData)
	if orderID.Uint64() != 1 {
		t.Fatalf("id %s", orderID)
	}
	cancel := append([]byte{ObMethodCancel}, u256Pad32(orderID)...)
	res, err = exec.ApplyMessage(Message{
		From: maker, To: &ob, Value: uint256.NewInt(0),
		GasLimit: 100_000, GasPrice: big.NewInt(0), Data: cancel,
	})
	if err != nil || res.Failed {
		t.Fatalf("cancel: %v %v", err, res.Err)
	}
	if statedb.GetBalance(maker).Cmp(before) != 0 {
		t.Fatalf("maker balance after cancel want %s got %s", before, statedb.GetBalance(maker))
	}
}

func TestOrderbook_PlaceSellFill_WithERC20(t *testing.T) {
	exec, _, maker, taker, token, ob, parsed := setupOrderbook(t)
	price := uint256.NewInt(1e18)
	baseAmt := uint256.NewInt(10)

	approveToken(t, exec, parsed, maker, token, ob, baseAmt)

	res, err := exec.ApplyMessage(Message{
		From: maker, To: &ob, Value: uint256.NewInt(0),
		GasLimit: 300_000, GasPrice: big.NewInt(0),
		Data: packPlace(1, token, price, baseAmt),
	})
	if err != nil || res.Failed {
		t.Fatalf("place sell: %v %v", err, res.Err)
	}
	orderID := new(uint256.Int).SetBytes(res.ReturnData)

	takerTokBefore := tokenBalanceOf(t, exec, parsed, token, taker)
	fill := append([]byte{ObMethodFill}, u256Pad32(orderID)...)
	fill = append(fill, u256Pad32(uint256.NewInt(4))...)
	res, err = exec.ApplyMessage(Message{
		From: taker, To: &ob, Value: uint256.NewInt(4),
		GasLimit: 400_000, GasPrice: big.NewInt(0), Data: fill,
	})
	if err != nil || res.Failed {
		t.Fatalf("fill: %v %v ret=%x", err, res.Err, res.ReturnData)
	}
	if tokenBalanceOf(t, exec, parsed, token, taker).Uint64() != takerTokBefore.Uint64()+4 {
		t.Fatal("taker did not receive base")
	}
	if len(res.ReturnData) != 64 {
		t.Fatalf("fill return len %d", len(res.ReturnData))
	}
	baseFilled := new(uint256.Int).SetBytes(res.ReturnData[0:32])
	quotePaid := new(uint256.Int).SetBytes(res.ReturnData[32:64])
	if baseFilled.Uint64() != 4 || quotePaid.Uint64() != 4 {
		t.Fatalf("filled %s quote %s", baseFilled, quotePaid)
	}
}

func TestOrderbook_PartialFillAndGetOrder(t *testing.T) {
	exec, _, maker, taker, token, ob, parsed := setupOrderbook(t)
	price := uint256.NewInt(1e18)
	baseAmt := uint256.NewInt(10)
	approveToken(t, exec, parsed, maker, token, ob, baseAmt)
	res, err := exec.ApplyMessage(Message{
		From: maker, To: &ob, Value: uint256.NewInt(0),
		GasLimit: 300_000, GasPrice: big.NewInt(0),
		Data: packPlace(1, token, price, baseAmt),
	})
	if err != nil || res.Failed {
		t.Fatalf("place: %v %v", err, res.Err)
	}
	orderID := new(uint256.Int).SetBytes(res.ReturnData)

	fill := append([]byte{ObMethodFill}, u256Pad32(orderID)...)
	fill = append(fill, u256Pad32(uint256.NewInt(3))...)
	res, err = exec.ApplyMessage(Message{
		From: taker, To: &ob, Value: uint256.NewInt(3),
		GasLimit: 400_000, GasPrice: big.NewInt(0), Data: fill,
	})
	if err != nil || res.Failed {
		t.Fatalf("fill: %v %v", err, res.Err)
	}

	get := append([]byte{ObMethodGetOrder}, u256Pad32(orderID)...)
	res, err = exec.ApplyMessage(Message{
		From: maker, To: &ob, Value: uint256.NewInt(0),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Data: get,
	})
	if err != nil || res.Failed {
		t.Fatalf("getOrder: %v %v", err, res.Err)
	}
	if len(res.ReturnData) != 9*32 {
		t.Fatalf("getOrder len %d", len(res.ReturnData))
	}
	baseOpen := new(uint256.Int).SetBytes(res.ReturnData[160:192])
	status := new(uint256.Int).SetBytes(res.ReturnData[256:288])
	if baseOpen.Uint64() != 7 || status.Uint64() != 0 {
		t.Fatalf("open=%s status=%s", baseOpen, status)
	}
}

func TestOrderbook_FillCancelledReverts(t *testing.T) {
	exec, _, maker, taker, token, ob, _ := setupOrderbook(t)
	price := uint256.NewInt(1e18)
	base := uint256.NewInt(1e18)
	lock := uint256.NewInt(1e18)
	res, err := exec.ApplyMessage(Message{
		From: maker, To: &ob, Value: lock,
		GasLimit: 200_000, GasPrice: big.NewInt(0),
		Data: packPlace(0, token, price, base),
	})
	if err != nil || res.Failed {
		t.Fatalf("place: %v %v", err, res.Err)
	}
	orderID := new(uint256.Int).SetBytes(res.ReturnData)
	cancel := append([]byte{ObMethodCancel}, u256Pad32(orderID)...)
	res, err = exec.ApplyMessage(Message{
		From: maker, To: &ob, Value: uint256.NewInt(0),
		GasLimit: 100_000, GasPrice: big.NewInt(0), Data: cancel,
	})
	if err != nil || res.Failed {
		t.Fatalf("cancel: %v %v", err, res.Err)
	}
	// taker tries fill cancelled buy — needs base approve
	fill := append([]byte{ObMethodFill}, u256Pad32(orderID)...)
	fill = append(fill, u256Pad32(uint256.NewInt(1))...)
	res, err = exec.ApplyMessage(Message{
		From: taker, To: &ob, Value: uint256.NewInt(0),
		GasLimit: 200_000, GasPrice: big.NewInt(0), Data: fill,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed {
		t.Fatal("expected fill cancelled to fail")
	}
}

func TestOrderbook_BestBidAskViews(t *testing.T) {
	exec, _, maker, _, token, ob, parsed := setupOrderbook(t)
	// two buys
	for _, price := range []*uint256.Int{uint256.NewInt(100), uint256.NewInt(200)} {
		base := uint256.NewInt(1e18)
		lock := nativeQuoteLock(price, base)
		res, err := exec.ApplyMessage(Message{
			From: maker, To: &ob, Value: lock,
			GasLimit: 200_000, GasPrice: big.NewInt(0),
			Data: packPlace(0, token, price, base),
		})
		if err != nil || res.Failed {
			t.Fatalf("place buy: %v %v", err, res.Err)
		}
	}
	// two sells
	approveToken(t, exec, parsed, maker, token, ob, uint256.NewInt(2e18))
	for _, price := range []*uint256.Int{uint256.NewInt(300), uint256.NewInt(250)} {
		res, err := exec.ApplyMessage(Message{
			From: maker, To: &ob, Value: uint256.NewInt(0),
			GasLimit: 300_000, GasPrice: big.NewInt(0),
			Data: packPlace(1, token, price, uint256.NewInt(1e18)),
		})
		if err != nil || res.Failed {
			t.Fatalf("place sell: %v %v", err, res.Err)
		}
	}

	bidIn := append([]byte{ObMethodBestBid}, token[:]...)
	res, err := exec.ApplyMessage(Message{
		From: maker, To: &ob, Value: uint256.NewInt(0),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Data: bidIn,
	})
	if err != nil || res.Failed {
		t.Fatalf("best bid: %v %v", err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData[0:32]).Uint64() != 200 {
		t.Fatalf("best bid price %x", res.ReturnData[0:32])
	}

	askIn := append([]byte{ObMethodBestAsk}, token[:]...)
	res, err = exec.ApplyMessage(Message{
		From: maker, To: &ob, Value: uint256.NewInt(0),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Data: askIn,
	})
	if err != nil || res.Failed {
		t.Fatalf("best ask: %v %v", err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData[0:32]).Uint64() != 250 {
		t.Fatalf("best ask price %x", res.ReturnData[0:32])
	}
}

func nativeQuoteLock(price, base *uint256.Int) *uint256.Int {
	// ceil(base * price / 1e18)
	prod := new(big.Int).Mul(base.ToBig(), price.ToBig())
	den := big.NewInt(1e18)
	prod.Add(prod, new(big.Int).Sub(den, big.NewInt(1)))
	prod.Div(prod, den)
	out, _ := uint256.FromBig(prod)
	return out
}
