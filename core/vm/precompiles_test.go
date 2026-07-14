package vm

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/params"
)

func TestNativeTransferPrecompile_CallWithValue(t *testing.T) {
	mdb := db.OpenTest(t)
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
	mdb := db.OpenTest(t)
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
	mdb := db.OpenTest(t)
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
	mdb := db.OpenTest(t)
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

	// Dual-vote double-sign evidence jails the signed offender (D3c).
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	offender := crypto.PubkeyToAddress(&key.PublicKey)
	fund2 := new(uint256.Int).Add(min, uint256.NewInt(1_000_000_000_000_000_000))
	statedb.SetBalance(offender, fund2)
	res, err = exec.ApplyMessage(Message{
		From: offender, To: &stakeAddr,
		Value: new(uint256.Int).Set(min), GasLimit: 200_000, GasPrice: big.NewInt(0),
		Data: []byte{StakeMethodBond},
	})
	if err != nil || res.Failed {
		t.Fatalf("bond offender: %v %v", err, res.Err)
	}
	var ha, hb types.Hash
	ha[0], hb[0] = 0x01, 0x02
	va := &consensus.Vote{Type: consensus.VotePrecommit, Height: 1, Round: 0, BlockHash: ha}
	vb := &consensus.Vote{Type: consensus.VotePrecommit, Height: 1, Round: 0, BlockHash: hb}
	if err := consensus.SignVote(va, key); err != nil {
		t.Fatal(err)
	}
	if err := consensus.SignVote(vb, key); err != nil {
		t.Fatal(err)
	}
	wire, err := consensus.EncodeDoubleSignEvidenceWire(va, vb)
	if err != nil {
		t.Fatal(err)
	}
	jailIn := append([]byte{StakeMethodJail}, wire...)
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: uint256.NewInt(0), GasLimit: 100_000, GasPrice: big.NewInt(0),
		Data: jailIn,
	})
	if err != nil || res.Failed {
		t.Fatalf("jail: %v %v", err, res.Err)
	}
	// Active set: caller remains; offender jailed out → count 1
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: uint256.NewInt(0), GasLimit: 50_000, GasPrice: big.NewInt(0),
		Data: []byte{StakeMethodActiveCount},
	})
	if err != nil || res.Failed {
		t.Fatal(err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData).Uint64() != 1 {
		t.Fatalf("active count after jail %x want 1", res.ReturnData)
	}
	qJail := append([]byte{StakeMethodIsJailed}, offender[:]...)
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: uint256.NewInt(0), GasLimit: 50_000, GasPrice: big.NewInt(0),
		Data: qJail,
	})
	if err != nil || res.Failed {
		t.Fatal(err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData).Uint64() != 1 {
		t.Fatal("offender should be jailed")
	}
}

func TestStakingBond_CreditsImmediateCallerViaTransfer(t *testing.T) {
	// Top-level EOA bond credits the payer recorded by Transfer (same as msg.sender for direct CALL).
	mdb := db.OpenTest(t)
	statedb := state.New(mdb)
	caller := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	min := uint256.MustFromBig(params.MinValidatorStakeWei())
	statedb.SetBalance(caller, new(uint256.Int).Add(min, uint256.NewInt(1e18)))
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
	if err != nil || res.Failed {
		t.Fatalf("bond: %v %v", err, res.Err)
	}
	q := append([]byte{StakeMethodGetSelfStake}, caller[:]...)
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &stakeAddr,
		Value: uint256.NewInt(0), GasLimit: 50_000, GasPrice: big.NewInt(0),
		Data: q,
	})
	if err != nil || res.Failed {
		t.Fatal(err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData).Cmp(min) != 0 {
		t.Fatalf("stake %x", res.ReturnData)
	}
}

// stakingForwarderRuntime forwards CALL(+value)+calldata to 0x102 and bubbles revert.
func stakingForwarderRuntime() []byte {
	addr := make([]byte, 20)
	copy(addr, StakingPrecompile[:])
	// See offset comments in S4 notes: JUMPDEST at 0x2b.
	out := []byte{
		0x36, 0x5f, 0x5f, 0x37, // calldatacopy(0,0,calldatasize)
		0x5f, 0x5f, 0x36, 0x5f, 0x34, // retLen retOff argLen argOff callvalue
		0x73, // PUSH20
	}
	out = append(out, addr...)
	out = append(out,
		0x5a, 0xf1, // GAS CALL
		0x3d, 0x5f, 0x5f, 0x3e, // returndatacopy(0,0,returndatasize)
		0x15,       // ISZERO
		0x60, 0x2b, // PUSH1 JUMPDEST
		0x57,       // JUMPI → revert path
		0x3d, 0x5f, 0xf3, // RETURN
		0x5b,             // JUMPDEST
		0x3d, 0x5f, 0xfd, // REVERT
	)
	return out
}

func stakingForwarderDeployCode() []byte {
	rt := stakingForwarderRuntime()
	if len(rt) > 255 {
		panic("forwarder runtime too long")
	}
	init := []byte{
		0x60, byte(len(rt)),
		0x80,
		0x60, 0x0b,
		0x60, 0x00,
		0x39,
		0x60, 0x00,
		0xf3,
	}
	return append(init, rt...)
}

func stakingTestExec(t *testing.T, statedb *state.StateDB, time uint64) *Executor {
	t.Helper()
	exec := NewExecutor(statedb, BlockContext{
		Number: 1, Time: time, GasLimit: 30_000_000, BaseFee: big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2205),
	})
	exec.EnableDewPrecompiles(true)
	exec.EnableStaking(true)
	cfg := native.DefaultStakingConfig()
	cfg.MinSelfStake = big.NewInt(1000)
	cfg.UnbondSeconds = 100
	exec.SetStakingConfig(cfg)
	return exec
}

func stakeAddr() crypto.Address {
	var a crypto.Address
	copy(a[:], StakingPrecompile[:])
	return a
}

func TestStakingPrecompile_BondUnbondWithdraw(t *testing.T) {
	mdb := db.OpenTest(t)
	statedb := state.New(mdb)
	caller := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	const bondAmt uint64 = 5000
	statedb.SetBalance(caller, uint256.NewInt(1_000_000_000_000_000_000))
	const t0 uint64 = 1_000
	exec := stakingTestExec(t, statedb, t0)
	to := stakeAddr()

	// Bond
	res, err := exec.ApplyMessage(Message{
		From: caller, To: &to, Value: uint256.NewInt(bondAmt),
		GasLimit: 200_000, GasPrice: big.NewInt(0), Data: []byte{StakeMethodBond},
	})
	if err != nil || res.Failed {
		t.Fatalf("bond: %v %v", err, res.Err)
	}

	// Unbond half
	unbondIn := append([]byte{StakeMethodUnbond}, make([]byte, 32)...)
	uint256.NewInt(bondAmt / 2).WriteToSlice(unbondIn[1:])
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &to, Value: uint256.NewInt(0),
		GasLimit: 100_000, GasPrice: big.NewInt(0), Data: unbondIn,
	})
	if err != nil || res.Failed {
		t.Fatalf("unbond: %v %v", err, res.Err)
	}

	// Early withdraw fails
	res, err = exec.ApplyMessage(Message{
		From: caller, To: &to, Value: uint256.NewInt(0),
		GasLimit: 100_000, GasPrice: big.NewInt(0), Data: []byte{StakeMethodWithdraw},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed {
		t.Fatal("expected early withdraw failure")
	}

	// After unbond period
	exec2 := stakingTestExec(t, statedb, t0+100)
	balBefore := statedb.GetBalance(caller).Clone()
	res, err = exec2.ApplyMessage(Message{
		From: caller, To: &to, Value: uint256.NewInt(0),
		GasLimit: 100_000, GasPrice: big.NewInt(0), Data: []byte{StakeMethodWithdraw},
	})
	if err != nil || res.Failed {
		t.Fatalf("withdraw: %v %v", err, res.Err)
	}
	got := new(uint256.Int).SetBytes(res.ReturnData)
	if got.Uint64() != bondAmt/2 {
		t.Fatalf("withdraw return %s", got)
	}
	if statedb.GetBalance(caller).Uint64() != balBefore.Uint64()+bondAmt/2 {
		t.Fatalf("balance after withdraw caller=%s before=%s", statedb.GetBalance(caller), balBefore)
	}

	// Remaining self-stake
	q := append([]byte{StakeMethodGetSelfStake}, caller[:]...)
	res, err = exec2.ApplyMessage(Message{
		From: caller, To: &to, Value: uint256.NewInt(0),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Data: q,
	})
	if err != nil || res.Failed {
		t.Fatal(err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData).Uint64() != bondAmt/2 {
		t.Fatalf("remaining stake %x", res.ReturnData)
	}
}

func TestStakingUnbondWithdraw_ActorIsTxOrigin_NestedForwarder(t *testing.T) {
	// S4 fail-closed: nested zero-value unbond/withdraw attribute to tx.origin, not the forwarder.
	mdb := db.OpenTest(t)
	statedb := state.New(mdb)
	origin := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	const bondAmt uint64 = 5000
	statedb.SetBalance(origin, uint256.NewInt(1_000_000_000_000_000_000))
	const t0 uint64 = 2_000
	exec := stakingTestExec(t, statedb, t0)
	to := stakeAddr()

	// Deploy CALL forwarder to 0x102.
	deploy, err := exec.ApplyMessage(Message{
		From: origin, To: nil, Value: uint256.NewInt(0),
		GasLimit: 500_000, GasPrice: big.NewInt(0), Data: stakingForwarderDeployCode(),
	})
	if err != nil || deploy.Failed || deploy.ContractAddress == nil {
		t.Fatalf("deploy forwarder: %v %v", err, deploy)
	}
	fwd := *deploy.ContractAddress

	// Nested bond with value: credits forwarder (immediate CALL payer), not origin alone.
	bondRes, err := exec.ApplyMessage(Message{
		From: origin, To: &fwd, Value: uint256.NewInt(bondAmt),
		GasLimit: 300_000, GasPrice: big.NewInt(0), Data: []byte{StakeMethodBond},
	})
	if err != nil || bondRes.Failed {
		t.Fatalf("nested bond: %v %v", err, bondRes.Err)
	}
	qFwd := append([]byte{StakeMethodGetSelfStake}, fwd[:]...)
	res, err := exec.ApplyMessage(Message{
		From: origin, To: &to, Value: uint256.NewInt(0),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Data: qFwd,
	})
	if err != nil || res.Failed {
		t.Fatal(err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData).Uint64() != bondAmt {
		t.Fatalf("forwarder stake %x want %d (nested bond should credit contract)", res.ReturnData, bondAmt)
	}
	qOrig := append([]byte{StakeMethodGetSelfStake}, origin[:]...)
	res, err = exec.ApplyMessage(Message{
		From: origin, To: &to, Value: uint256.NewInt(0),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Data: qOrig,
	})
	if err != nil || res.Failed {
		t.Fatal(err, res.Err)
	}
	if !new(uint256.Int).SetBytes(res.ReturnData).IsZero() {
		t.Fatalf("origin should have 0 self-stake after nested bond to forwarder, got %x", res.ReturnData)
	}

	// Nested unbond via forwarder: acts on tx.origin (0 stake) → fail; forwarder stake unchanged.
	unbondIn := append([]byte{StakeMethodUnbond}, make([]byte, 32)...)
	uint256.NewInt(bondAmt).WriteToSlice(unbondIn[1:])
	res, err = exec.ApplyMessage(Message{
		From: origin, To: &fwd, Value: uint256.NewInt(0),
		GasLimit: 200_000, GasPrice: big.NewInt(0), Data: unbondIn,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed {
		t.Fatal("nested unbond should fail: actor is origin with zero stake (fail-closed)")
	}
	res, err = exec.ApplyMessage(Message{
		From: origin, To: &to, Value: uint256.NewInt(0),
		GasLimit: 50_000, GasPrice: big.NewInt(0), Data: qFwd,
	})
	if err != nil || res.Failed {
		t.Fatal(err, res.Err)
	}
	if new(uint256.Int).SetBytes(res.ReturnData).Uint64() != bondAmt {
		t.Fatalf("forwarder stake should remain %d, got %x", bondAmt, res.ReturnData)
	}

	// Direct unbond as origin after origin self-bonds: works (control).
	res, err = exec.ApplyMessage(Message{
		From: origin, To: &to, Value: uint256.NewInt(bondAmt),
		GasLimit: 200_000, GasPrice: big.NewInt(0), Data: []byte{StakeMethodBond},
	})
	if err != nil || res.Failed {
		t.Fatalf("origin bond: %v %v", err, res.Err)
	}
	res, err = exec.ApplyMessage(Message{
		From: origin, To: &to, Value: uint256.NewInt(0),
		GasLimit: 100_000, GasPrice: big.NewInt(0), Data: unbondIn,
	})
	if err != nil || res.Failed {
		t.Fatalf("direct origin unbond: %v %v", err, res.Err)
	}
}
