package node_test

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/vm"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/params"
)

func orderbookPtr() *crypto.Address {
	a := crypto.MustHexToAddress("0x0000000000000000000000000000000000000101")
	return &a
}

func orderbookCommon() common.Address {
	return common.BytesToAddress([]byte{0x01, 0x01})
}

func padU256(v *uint256.Int) []byte {
	if v == nil {
		v = uint256.NewInt(0)
	}
	return common.LeftPadBytes(v.ToBig().Bytes(), 32)
}

func packOBPlace(side byte, token crypto.Address, price, base *uint256.Int) []byte {
	out := make([]byte, 0, 86)
	out = append(out, vm.ObMethodPlace, side)
	out = append(out, token[:]...)
	out = append(out, padU256(price)...)
	out = append(out, padU256(base)...)
	return out
}

func signCall101(t *testing.T, privHex string, chainID *big.Int, nonce uint64, value *big.Int, data []byte, gas uint64) []byte {
	t.Helper()
	key, err := ethcrypto.HexToECDSA(privHex)
	if err != nil {
		t.Fatal(err)
	}
	to := orderbookCommon()
	if gas == 0 {
		gas = 500_000
	}
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    value,
		Gas:      gas,
		GasPrice: big.NewInt(1_000_000_000),
		Data:     data,
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(chainID), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func signTokenCall(t *testing.T, privHex string, chainID *big.Int, nonce uint64, token common.Address, data []byte) []byte {
	t.Helper()
	key, err := ethcrypto.HexToECDSA(privHex)
	if err != nil {
		t.Fatal(err)
	}
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		To:       &token,
		Value:    big.NewInt(0),
		Gas:      200_000,
		GasPrice: big.NewInt(1_000_000_000),
		Data:     data,
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(chainID), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func deployMockToken(t *testing.T, n *node.Node, privHex string, nonce uint64) crypto.Address {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(vm.TokenABI))
	if err != nil {
		t.Fatal(err)
	}
	bin, err := hex.DecodeString(vm.TokenCreationBytecode)
	if err != nil {
		t.Fatal(err)
	}
	supply := new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18))
	ctor, err := parsed.Pack("", supply)
	if err != nil {
		t.Fatal(err)
	}
	data := append(append([]byte{}, bin...), ctor...)
	key, err := ethcrypto.HexToECDSA(privHex)
	if err != nil {
		t.Fatal(err)
	}
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      3_000_000,
		Data:     data,
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(n.ChainID()), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	h, err := n.SendRawTransaction(raw)
	if err != nil {
		t.Fatalf("deploy token: %v", err)
	}
	rcpt := n.GetReceipt(h)
	if rcpt == nil || rcpt.Status != 1 || rcpt.ContractAddress == nil {
		t.Fatalf("deploy receipt status=%v addr=%v", rcpt, rcpt)
	}
	return *rcpt.ContractAddress
}

// TestOrderbookLab_Scenario walks token deploy → approve → place sell → fill → getOrder
// → place buy → cancel on a node with SetNativeSwapEnabled(true).
// Lab-only: public-testnet default remains off.
func TestOrderbookLab_Scenario(t *testing.T) {
	if params.DefaultEnableNativeSwap {
		t.Fatal("public freeze expects native swap default off")
	}

	g, err := devnet.DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	n := node.OpenTest(t, g)
	if n.NativeSwapEnabled() {
		t.Fatal("node must default native swap off")
	}
	n.SetNativeSwapEnabled(true)

	chainID := n.ChainID()
	maker := addrFromPriv(t, devnet.PrivHex0)
	taker := addrFromPriv(t, devnet.PrivHex1)
	ob := orderbookPtr()

	// Flag-off path is covered by fresh node at end; with flag on, methods work.
	token := deployMockToken(t, n, devnet.PrivHex0, 0)
	tokenCommon := common.BytesToAddress(token[:])

	parsed, err := abi.JSON(strings.NewReader(vm.TokenABI))
	if err != nil {
		t.Fatal(err)
	}

	// Transfer base to taker for later buy-fill path not used; sell path needs maker approve.
	baseAmt := uint256.NewInt(10)
	price := uint256.NewInt(1e18) // 1 DEW per 1e18 base unit → quote = base amount for small sizes

	// Approve 0x101 to pull maker base for sell place.
	approveData, err := parsed.Pack("approve", orderbookCommon(), baseAmt.ToBig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.SendRawTransaction(signTokenCall(t, devnet.PrivHex0, chainID, 1, tokenCommon, approveData)); err != nil {
		t.Fatalf("approve: %v", err)
	}

	// Place sell: side=1, no CALLVALUE.
	placeData := packOBPlace(1, token, price, baseAmt)
	hPlace, err := n.SendRawTransaction(signCall101(t, devnet.PrivHex0, chainID, 2, big.NewInt(0), placeData, 500_000))
	if err != nil {
		t.Fatalf("place sell: %v", err)
	}
	rcptPlace := n.GetReceipt(hPlace)
	if rcptPlace == nil || rcptPlace.Status != 1 {
		t.Fatalf("place receipt: %+v", rcptPlace)
	}

	// eth_call GetOrder via BestAsk first to learn order id if return data not on receipt.
	// Place returns orderId in return data — recover via eth_call BestAsk.
	bestData := append([]byte{vm.ObMethodBestAsk}, token[:]...)
	bestOut, err := n.Call(vm.Message{
		From: maker, To: ob, Data: bestData,
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("bestAsk: %v", err)
	}
	if len(bestOut) < 64 {
		t.Fatalf("bestAsk len %d", len(bestOut))
	}
	askPrice := new(uint256.Int).SetBytes(bestOut[0:32])
	orderID := new(uint256.Int).SetBytes(bestOut[32:64])
	if askPrice.Cmp(price) != 0 || orderID.IsZero() {
		t.Fatalf("bestAsk price=%s id=%s", askPrice, orderID)
	}

	// Partial fill: taker pays 4 DEW, takes 4 base.
	fillBase := uint256.NewInt(4)
	fillData := append([]byte{vm.ObMethodFill}, padU256(orderID)...)
	fillData = append(fillData, padU256(fillBase)...)
	if _, err := n.SendRawTransaction(signCall101(t, devnet.PrivHex1, chainID, 0, big.NewInt(4), fillData, 500_000)); err != nil {
		t.Fatalf("fill: %v", err)
	}

	// GetOrder: base open should be 6, status open (0).
	getData := append([]byte{vm.ObMethodGetOrder}, padU256(orderID)...)
	getOut, err := n.Call(vm.Message{
		From: maker, To: ob, Data: getData,
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("getOrder: %v", err)
	}
	if len(getOut) != 9*32 {
		t.Fatalf("getOrder len %d", len(getOut))
	}
	baseOpen := new(uint256.Int).SetBytes(getOut[160:192])
	status := new(uint256.Int).SetBytes(getOut[256:288])
	if baseOpen.Uint64() != 6 || status.Uint64() != 0 {
		t.Fatalf("after fill open=%s status=%s want open=6 status=0", baseOpen, status)
	}

	// Taker base balance increased by 4.
	balData, err := parsed.Pack("balanceOf", common.BytesToAddress(taker[:]))
	if err != nil {
		t.Fatal(err)
	}
	balOut, err := n.Call(vm.Message{
		From: taker, To: &token, Data: balData,
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("balanceOf taker: %v", err)
	}
	if new(uint256.Int).SetBytes(balOut).Uint64() != 4 {
		t.Fatalf("taker base bal %x want 4", balOut)
	}

	// Place buy + cancel: maker locks 2 DEW for 1e18 base @ price 2e18.
	buyPrice := uint256.NewInt(2e18)
	buyBase := uint256.NewInt(1e18)
	lock := uint256.NewInt(2e18)
	buyData := packOBPlace(0, token, buyPrice, buyBase)
	if _, err := n.SendRawTransaction(signCall101(t, devnet.PrivHex0, chainID, 3, lock.ToBig(), buyData, 300_000)); err != nil {
		t.Fatalf("place buy: %v", err)
	}
	bidOut, err := n.Call(vm.Message{
		From: maker, To: ob, Data: append([]byte{vm.ObMethodBestBid}, token[:]...),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("bestBid: %v", err)
	}
	buyID := new(uint256.Int).SetBytes(bidOut[32:64])
	if buyID.IsZero() {
		t.Fatal("expected bestBid order id")
	}
	cancelData := append([]byte{vm.ObMethodCancel}, padU256(buyID)...)
	if _, err := n.SendRawTransaction(signCall101(t, devnet.PrivHex0, chainID, 4, big.NewInt(0), cancelData, 200_000)); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	// BestBid should clear after cancel.
	bid2, err := n.Call(vm.Message{
		From: maker, To: ob, Data: append([]byte{vm.ObMethodBestBid}, token[:]...),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err != nil {
		t.Fatalf("bestBid after cancel: %v", err)
	}
	if !new(uint256.Int).SetBytes(bid2[32:64]).IsZero() {
		t.Fatalf("bestBid should be empty after cancel: %x", bid2)
	}

	// Fresh node keeps native swap off (public / private default).
	n2 := node.OpenTest(t, g)
	if n2.NativeSwapEnabled() {
		t.Fatal("fresh node must keep native swap off by default")
	}
	// Flag-off method call reverts.
	_, err = n2.Call(vm.Message{
		From: maker, To: orderbookPtr(), Data: []byte{vm.ObMethodBestAsk},
		GasLimit: 50_000, GasPrice: big.NewInt(0), Value: uint256.NewInt(0),
	})
	if err == nil {
		t.Fatal("expected flag-off call to fail")
	}
}
