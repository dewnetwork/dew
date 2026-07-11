package vm

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/params"
)

func TestNativeTransferPrecompile_CallWithValue(t *testing.T) {
	mdb := db.NewMemoryDB()
	defer mdb.Close()
	statedb := state.New(mdb)

	caller := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	recipient := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	coinbase := crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0")
	statedb.SetBalance(caller, uint256.NewInt(1_000_000_000_000_000_000)) // 1 eth

	exec := NewExecutor(statedb, BlockContext{
		Number:   1,
		Time:     1_700_000_000,
		GasLimit: 30_000_000,
		BaseFee:  big.NewInt(0),
		Coinbase: coinbase,
		ChainID:  big.NewInt(2205),
	})
	exec.EnableDewPrecompiles(true)

	var precompileAddr crypto.Address
	copy(precompileAddr[:], NativeTransferPrecompile[:])

	amount := uint256.NewInt(1_000_000_000_000_000) // 0.001 eth
	res, err := exec.ApplyMessage(Message{
		From:     caller,
		To:       &precompileAddr,
		Value:    amount,
		GasLimit: 100_000,
		GasPrice: big.NewInt(0),
		Data:     recipient[:],
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed {
		t.Fatalf("call failed: %v ret=%x", res.Err, res.ReturnData)
	}
	if statedb.GetBalance(recipient).Cmp(amount) != 0 {
		t.Fatalf("recipient bal = %s want %s", statedb.GetBalance(recipient), amount)
	}
	// precompile address should be empty after forward
	if !statedb.GetBalance(precompileAddr).IsZero() {
		t.Fatalf("precompile still holds %s", statedb.GetBalance(precompileAddr))
	}
	if res.UsedGas < NativeTransferGas {
		t.Fatalf("used gas %d < fixed %d", res.UsedGas, NativeTransferGas)
	}
}

func TestNativeTransferPrecompile_Disabled(t *testing.T) {
	mdb := db.NewMemoryDB()
	defer mdb.Close()
	statedb := state.New(mdb)
	caller := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	recipient := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	statedb.SetBalance(caller, uint256.NewInt(1_000_000_000_000_000_000))

	exec := NewExecutor(statedb, BlockContext{
		Number: 1, Time: 1, GasLimit: 30_000_000, BaseFee: big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2205),
	})
	exec.EnableDewPrecompiles(false)

	var precompileAddr crypto.Address
	copy(precompileAddr[:], NativeTransferPrecompile[:])

	res, err := exec.ApplyMessage(Message{
		From: caller, To: &precompileAddr,
		Value: uint256.NewInt(1000), GasLimit: 100_000, GasPrice: big.NewInt(0),
		Data: recipient[:],
	})
	if err != nil {
		t.Fatal(err)
	}
	// Without precompile, CALL to empty account just transfers value and succeeds
	// with empty return — value stays at 0x100, not forwarded.
	if statedb.GetBalance(recipient).Uint64() != 0 {
		t.Fatal("recipient should not receive when precompile disabled")
	}
	if statedb.GetBalance(precompileAddr).Uint64() != 1000 {
		t.Fatalf("value should sit at empty account, got %s", statedb.GetBalance(precompileAddr))
	}
	_ = res
}

func TestStakingPrecompile_DisabledReverts(t *testing.T) {
	mdb := db.NewMemoryDB()
	defer mdb.Close()
	statedb := state.New(mdb)
	caller := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	statedb.SetBalance(caller, uint256.NewInt(1_000_000_000_000_000_000))

	exec := NewExecutor(statedb, BlockContext{
		Number: 1, Time: 1, GasLimit: 30_000_000, BaseFee: big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2205),
	})
	exec.EnableDewPrecompiles(true)
	// stakingEnabled default false

	var stakeAddr crypto.Address
	copy(stakeAddr[:], StakingPrecompile[:])
	res, err := exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: uint256.NewInt(0), GasLimit: 100_000, GasPrice: big.NewInt(0),
		Data: []byte{StakeMethodBond},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed {
		t.Fatal("expected staking disabled to fail")
	}
}

func TestStakingPrecompile_BondUnbondActiveSet(t *testing.T) {
	mdb := db.NewMemoryDB()
	defer mdb.Close()
	statedb := state.New(mdb)
	caller := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	min := uint256.MustFromBig(params.MinValidatorStakeWei())
	// fund min + gas room
	fund := new(uint256.Int).Add(min, uint256.NewInt(1_000_000_000_000_000_000))
	statedb.SetBalance(caller, fund)

	exec := NewExecutor(statedb, BlockContext{
		Number: 1, Time: 1, GasLimit: 30_000_000, BaseFee: big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2205),
	})
	exec.EnableDewPrecompiles(true)
	exec.EnableStaking(true)

	var stakeAddr crypto.Address
	copy(stakeAddr[:], StakingPrecompile[:])

	res, err := exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: new(uint256.Int).Set(min), GasLimit: 200_000, GasPrice: big.NewInt(0),
		Data: []byte{StakeMethodBond},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed {
		t.Fatalf("bond failed: %v", res.Err)
	}

	// query self stake
	q := append([]byte{StakeMethodGetSelfStake}, caller[:]...)
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: uint256.NewInt(0), GasLimit: 50_000, GasPrice: big.NewInt(0),
		Data: q,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed {
		t.Fatalf("get stake: %v", res.Err)
	}
	got := new(uint256.Int).SetBytes(res.ReturnData)
	if got.Cmp(min) != 0 {
		t.Fatalf("stake %s want %s", got, min)
	}

	// active set contains caller
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: uint256.NewInt(0), GasLimit: 50_000, GasPrice: big.NewInt(0),
		Data: []byte{StakeMethodActiveCount},
	})
	if err != nil || res.Failed {
		t.Fatalf("active count: %v %v", err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData).Uint64() != 1 {
		t.Fatalf("active count %x", res.ReturnData)
	}

	// jail with evidence
	var ev [32]byte
	ev[0] = 0xab
	jailIn := append([]byte{StakeMethodJail}, caller[:]...)
	jailIn = append(jailIn, ev[:]...)
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: uint256.NewInt(0), GasLimit: 100_000, GasPrice: big.NewInt(0),
		Data: jailIn,
	})
	if err != nil || res.Failed {
		t.Fatalf("jail: %v %v", err, res.Err)
	}
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: uint256.NewInt(0), GasLimit: 50_000, GasPrice: big.NewInt(0),
		Data: []byte{StakeMethodActiveCount},
	})
	if err != nil || res.Failed {
		t.Fatal(err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData).Uint64() != 0 {
		t.Fatal("jailed should leave active set")
	}
}
