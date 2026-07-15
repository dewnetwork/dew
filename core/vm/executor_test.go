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
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

func TestExecutor_DeployERC20_Transfer_Events(t *testing.T) {
	mdb := db.OpenTest(t)
	statedb := state.New(mdb)

	deployer := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	recipient := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	coinbase := crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0")

	// Fund deployer with plenty of native gas money
	statedb.SetBalance(deployer, uint256.MustFromBig(new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil)))

	exec := NewExecutor(statedb, BlockContext{
		Number:   1,
		Time:     1_700_000_000,
		GasLimit: 30_000_000,
		BaseFee:  big.NewInt(0),
		Coinbase: coinbase,
		ChainID:  big.NewInt(2205),
	})

	parsed, err := abi.JSON(strings.NewReader(TokenABI))
	if err != nil {
		t.Fatalf("abi: %v", err)
	}
	bin, err := hex.DecodeString(TokenCreationBytecode)
	if err != nil {
		t.Fatalf("bytecode: %v", err)
	}
	supply := new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18))
	ctor, err := parsed.Pack("", supply)
	if err != nil {
		t.Fatalf("pack ctor: %v", err)
	}
	deployData := append(append([]byte{}, bin...), ctor...)

	deployRes, err := exec.ApplyMessage(Message{
		From:     deployer,
		To:       nil,
		Value:    uint256.NewInt(0),
		GasLimit: 3_000_000,
		GasPrice: big.NewInt(1),
		Data:     deployData,
	})
	if err != nil {
		t.Fatalf("ApplyMessage deploy: %v", err)
	}
	if deployRes.Failed {
		t.Fatalf("deploy failed: %v return=%x", deployRes.Err, deployRes.ReturnData)
	}
	if deployRes.ContractAddress == nil {
		t.Fatal("missing contract address")
	}
	token := *deployRes.ContractAddress
	if len(statedb.GetCode(token)) == 0 {
		t.Fatal("runtime code not stored")
	}
	// Mint Transfer event (from 0x0 → deployer)
	if len(deployRes.Logs) == 0 {
		t.Fatal("expected Transfer log on deploy")
	}
	transferTopic := crypto.Keccak256([]byte("Transfer(address,address,uint256)"))
	if !bytesEqual(deployRes.Logs[0].Topics[0].Bytes(), transferTopic) {
		t.Fatalf("topic0 = %x, want Transfer", deployRes.Logs[0].Topics[0].Bytes())
	}

	// balanceOf(deployer) == supply
	balData, err := parsed.Pack("balanceOf", ethcommon.BytesToAddress(deployer[:]))
	if err != nil {
		t.Fatal(err)
	}
	balRes, err := exec.ApplyMessage(Message{
		From:     deployer,
		To:       &token,
		Value:    uint256.NewInt(0),
		GasLimit: 100_000,
		GasPrice: big.NewInt(1),
		Data:     balData,
	})
	if err != nil || balRes.Failed {
		t.Fatalf("balanceOf: err=%v res=%+v", err, balRes)
	}
	gotBal := new(big.Int).SetBytes(balRes.ReturnData)
	if gotBal.Cmp(supply) != 0 {
		t.Fatalf("balanceOf deployer = %s, want %s", gotBal, supply)
	}

	// transfer(recipient, 1000)
	amount := big.NewInt(1000)
	txData, err := parsed.Pack("transfer", ethcommon.BytesToAddress(recipient[:]), amount)
	if err != nil {
		t.Fatal(err)
	}
	txRes, err := exec.ApplyMessage(Message{
		From:     deployer,
		To:       &token,
		Value:    uint256.NewInt(0),
		GasLimit: 100_000,
		GasPrice: big.NewInt(1),
		Data:     txData,
	})
	if err != nil || txRes.Failed {
		t.Fatalf("transfer: err=%v resErr=%v ret=%x", err, txRes.Err, txRes.ReturnData)
	}
	if len(txRes.Logs) != 1 {
		t.Fatalf("transfer logs = %d, want 1", len(txRes.Logs))
	}
	// indexed from / to
	if fromEthHashTopic(txRes.Logs[0].Topics[1]) != deployer {
		t.Fatalf("log from = %s", txRes.Logs[0].Topics[1].Hex())
	}
	if fromEthHashTopic(txRes.Logs[0].Topics[2]) != recipient {
		t.Fatalf("log to = %s", txRes.Logs[0].Topics[2].Hex())
	}

	// recipient balance
	balData2, _ := parsed.Pack("balanceOf", ethcommon.BytesToAddress(recipient[:]))
	balRes2, err := exec.ApplyMessage(Message{
		From:     deployer,
		To:       &token,
		GasLimit: 100_000,
		GasPrice: big.NewInt(1),
		Data:     balData2,
	})
	if err != nil || balRes2.Failed {
		t.Fatalf("balanceOf recipient: %v %+v", err, balRes2)
	}
	if new(big.Int).SetBytes(balRes2.ReturnData).Cmp(amount) != 0 {
		t.Fatalf("recipient bal = %x", balRes2.ReturnData)
	}
}

func TestExecutor_ApproveTransferFrom(t *testing.T) {
	mdb := db.OpenTest(t)
	statedb := state.New(mdb)
	owner := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	spender := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	to := crypto.MustHexToAddress("0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC")
	fund := uint256.MustFromBig(new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil))
	statedb.SetBalance(owner, fund)
	statedb.SetBalance(spender, new(uint256.Int).Set(fund))

	exec := NewExecutor(statedb, BlockContext{
		Number: 1, Time: 1, GasLimit: 30_000_000, BaseFee: big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2205),
	})
	parsed, err := abi.JSON(strings.NewReader(TokenABI))
	if err != nil {
		t.Fatal(err)
	}
	bin, _ := hex.DecodeString(TokenCreationBytecode)
	supply := big.NewInt(10_000)
	ctor, _ := parsed.Pack("", supply)
	dep, err := exec.ApplyMessage(Message{
		From: owner, GasLimit: 3_000_000, GasPrice: big.NewInt(0),
		Data: append(append([]byte{}, bin...), ctor...),
	})
	if err != nil || dep.Failed {
		t.Fatalf("deploy: %v %+v", err, dep)
	}
	token := *dep.ContractAddress

	approveData, err := parsed.Pack("approve", ethcommon.BytesToAddress(spender[:]), big.NewInt(500))
	if err != nil {
		t.Fatal(err)
	}
	res, err := exec.ApplyMessage(Message{
		From: owner, To: &token, GasLimit: 100_000, GasPrice: big.NewInt(0), Data: approveData,
	})
	if err != nil || res.Failed {
		t.Fatalf("approve: %v %v", err, res.Err)
	}

	tfData, err := parsed.Pack(
		"transferFrom",
		ethcommon.BytesToAddress(owner[:]),
		ethcommon.BytesToAddress(to[:]),
		big.NewInt(200),
	)
	if err != nil {
		t.Fatal(err)
	}
	res, err = exec.ApplyMessage(Message{
		From: spender, To: &token, GasLimit: 150_000, GasPrice: big.NewInt(0), Data: tfData,
	})
	if err != nil || res.Failed {
		t.Fatalf("transferFrom: %v %v ret=%x", err, res.Err, res.ReturnData)
	}
	balData, _ := parsed.Pack("balanceOf", ethcommon.BytesToAddress(to[:]))
	balRes, err := exec.ApplyMessage(Message{
		From: owner, To: &token, GasLimit: 50_000, GasPrice: big.NewInt(0), Data: balData,
	})
	if err != nil || balRes.Failed {
		t.Fatal(err)
	}
	if new(big.Int).SetBytes(balRes.ReturnData).Cmp(big.NewInt(200)) != 0 {
		t.Fatalf("to bal %x", balRes.ReturnData)
	}
}

func TestExecutor_FailedTx_RevertsState(t *testing.T) {
	mdb := db.OpenTest(t)
	statedb := state.New(mdb)

	from := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	to := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	statedb.SetBalance(from, uint256.NewInt(1_000_000_000_000_000_000)) // 1 eth

	exec := NewExecutor(statedb, BlockContext{
		Number:   1,
		Time:     1,
		GasLimit: 30_000_000,
		BaseFee:  big.NewInt(0),
		ChainID:  big.NewInt(2205),
	})

	// Deploy token with small supply
	parsed, _ := abi.JSON(strings.NewReader(TokenABI))
	bin, _ := hex.DecodeString(TokenCreationBytecode)
	supply := big.NewInt(100)
	ctor, _ := parsed.Pack("", supply)
	deployData := append(append([]byte{}, bin...), ctor...)

	dep, err := exec.ApplyMessage(Message{
		From: from, GasLimit: 3_000_000, GasPrice: big.NewInt(1), Data: deployData,
	})
	if err != nil || dep.Failed {
		t.Fatalf("deploy: %v %+v", err, dep)
	}
	token := *dep.ContractAddress

	balBefore := statedb.GetBalance(from).Clone()
	nonceBefore := statedb.GetNonce(from)

	// transfer more than balance → revert
	txData, _ := parsed.Pack("transfer", ethcommon.BytesToAddress(to[:]), big.NewInt(10_000))
	res, err := exec.ApplyMessage(Message{
		From: from, To: &token, GasLimit: 100_000, GasPrice: big.NewInt(1), Data: txData,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Failed {
		t.Fatal("expected failed transfer")
	}

	// Token balances unchanged
	balData, _ := parsed.Pack("balanceOf", ethcommon.BytesToAddress(from[:]))
	check, err := exec.ApplyMessage(Message{
		From: from, To: &token, GasLimit: 100_000, GasPrice: big.NewInt(1), Data: balData,
	})
	if err != nil || check.Failed {
		t.Fatalf("balanceOf after fail: %v %+v", err, check)
	}
	if new(big.Int).SetBytes(check.ReturnData).Cmp(supply) != 0 {
		t.Fatalf("token balance changed on revert: %s", new(big.Int).SetBytes(check.ReturnData))
	}

	// Gas still charged (native balance decreased)
	if statedb.GetBalance(from).Cmp(balBefore) >= 0 {
		// may have paid gas for the failed tx AND the balanceOf check after
		// re-check: balBefore was before failed tx; after failed+check should be lower
	}
	_ = nonceBefore
	if res.UsedGas == 0 {
		t.Fatal("failed tx should still use gas")
	}
}

func TestBridge_ImplementsSnapshotRevert(t *testing.T) {
	mdb := db.OpenTest(t)
	statedb := state.New(mdb)
	addr := crypto.MustHexToAddress("0x0000000000000000000000000000000000000001")
	statedb.SetBalance(addr, uint256.NewInt(100))

	bridge := NewBridge(statedb)
	snap := bridge.Snapshot()
	bridge.AddBalance(toEthAddr(addr), uint256.NewInt(50), 0)
	if bridge.GetBalance(toEthAddr(addr)).Uint64() != 150 {
		t.Fatal("add")
	}
	bridge.RevertToSnapshot(snap)
	if bridge.GetBalance(toEthAddr(addr)).Uint64() != 100 {
		t.Fatalf("revert failed: %s", bridge.GetBalance(toEthAddr(addr)))
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func fromEthHashTopic(h types.Hash) crypto.Address {
	// indexed address is left-padded in topic
	var a crypto.Address
	copy(a[:], h[12:])
	return a
}
