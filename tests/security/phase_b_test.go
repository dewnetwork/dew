// Package security holds Phase B adversarial / fail-closed tests.
// Run: go test ./tests/security/ -count=1
package security

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/core/vm"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/params"
)

func TestSecurity_DewTxDomainNotEVMReplay(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	tx := types.NewDewTx(big.NewInt(2026), 0, sender, recv, uint256.NewInt(1), params.DefaultDewTxFeeWei, nil, nil)
	if err := types.SignDewTx(tx, key); err != nil {
		t.Fatal(err)
	}
	// Mutate domain-critical field after sign → recover must fail
	// (SigningHash is recomputed from fields; V,R,S stay bound to original.)
	tx2 := *tx
	tx2.ChainID = big.NewInt(1)
	if _, err := tx2.RecoverSender(); err == nil {
		t.Fatal("expected recover failure after chainId tamper")
	}
}

func TestSecurity_DewTxWrongChainID_Node(t *testing.T) {
	n := newTestNode(t)
	// Anvil #0 — funded in genesis alloc
	key, err := crypto.ToECDSA(mustDecodeHex("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"))
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	tx := types.NewDewTx(big.NewInt(999), 0, sender, recv, uint256.NewInt(1), params.DefaultDewTxFeeWei, nil, nil)
	if err := types.SignDewTx(tx, key); err != nil {
		t.Fatal(err)
	}
	raw, _ := tx.MarshalBinary()
	if _, err := n.SendDewRawTransaction(raw); err == nil {
		t.Fatal("expected wrong chain id rejection")
	}
}

func TestSecurity_DewTxRejectsEVMPrefix(t *testing.T) {
	var tx types.DewTx
	if err := tx.UnmarshalBinary([]byte{0x02, 0xc0}); err == nil {
		t.Fatal("EVM typed tx must not decode as DewTx")
	}
}

func TestSecurity_AccessListFailClosedNoMutation(t *testing.T) {
	mdb := db.NewMemoryDB()
	t.Cleanup(func() { mdb.Close() })
	st := state.New(mdb)
	key, _ := crypto.GenerateKey()
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	third := crypto.MustHexToAddress("0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC")
	sink := crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0")
	st.SetBalance(sender, uint256.NewInt(params.DefaultDewTxFeeWei*5))
	before, _ := st.IntermediateRoot()

	payload := make([]byte, 1+20+32)
	payload[0] = native.PayloadCredit
	copy(payload[1:21], third[:])
	b32 := uint256.NewInt(1).Bytes32()
	copy(payload[21:], b32[:])
	tx := types.NewDewTx(big.NewInt(2026), 0, sender, recv, uint256.NewInt(0), params.DefaultDewTxFeeWei, payload, nil)
	_ = types.SignDewTx(tx, key)
	res, err := native.NewExecutor(st, sink).ApplyDewTx(tx)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed {
		t.Fatal("expected fail-closed")
	}
	after, _ := st.IntermediateRoot()
	if before != after {
		t.Fatal("state mutated on fail-closed access list")
	}
	if st.GetNonce(sender) != 0 {
		t.Fatal("nonce advanced on failure")
	}
}

func TestSecurity_NativeDisabled(t *testing.T) {
	n := newTestNode(t)
	n.SetNativeEnabled(false)
	if _, err := n.SendDewRawTransaction([]byte{types.DewTxType, 0xc0}); err == nil {
		t.Fatal("expected native disabled error")
	}
}

func TestSecurity_PrecompileDisabledNoForward(t *testing.T) {
	mdb := db.NewMemoryDB()
	t.Cleanup(func() { mdb.Close() })
	st := state.New(mdb)
	caller := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	recipient := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	st.SetBalance(caller, uint256.NewInt(1_000_000_000_000_000_000))
	exec := vm.NewExecutor(st, vm.BlockContext{
		Number: 1, Time: 1, GasLimit: 30_000_000, BaseFee: big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2026),
	})
	exec.EnableDewPrecompiles(false)
	var pre crypto.Address
	copy(pre[:], vm.NativeTransferPrecompile[:])
	_, err := exec.ApplyMessage(vm.Message{
		From: caller, To: &pre, Value: uint256.NewInt(1000),
		GasLimit: 100_000, GasPrice: big.NewInt(0), Data: recipient[:],
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.GetBalance(recipient).Uint64() != 0 {
		t.Fatal("value forwarded while precompile disabled")
	}
}

func TestSecurity_PrecompileBadInputReverts(t *testing.T) {
	mdb := db.NewMemoryDB()
	t.Cleanup(func() { mdb.Close() })
	st := state.New(mdb)
	caller := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	st.SetBalance(caller, uint256.NewInt(1_000_000_000_000_000_000))
	exec := vm.NewExecutor(st, vm.BlockContext{
		Number: 1, Time: 1, GasLimit: 30_000_000, BaseFee: big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2026),
	})
	exec.EnableDewPrecompiles(true)
	var pre crypto.Address
	copy(pre[:], vm.NativeTransferPrecompile[:])
	res, err := exec.ApplyMessage(vm.Message{
		From: caller, To: &pre, Value: uint256.NewInt(1000),
		GasLimit: 100_000, GasPrice: big.NewInt(0), Data: []byte{0x01, 0x02}, // not 20 bytes
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Failed {
		t.Fatal("expected revert on bad input")
	}
}

func TestSecurity_ParallelEquivalenceUnderConflict(t *testing.T) {
	mdb := db.NewMemoryDB()
	t.Cleanup(func() { mdb.Close() })
	base := state.New(mdb)
	key, _ := crypto.GenerateKey()
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	base.SetBalance(sender, uint256.NewInt(1_000_000))
	var msgs []vm.Message
	for i := 0; i < 8; i++ {
		k, _ := crypto.GenerateKey()
		to := crypto.PubkeyToAddress(&k.PublicKey)
		msgs = append(msgs, vm.Message{
			From: sender, To: &to, Value: uint256.NewInt(1),
			GasLimit: 100_000, GasPrice: big.NewInt(0),
		})
	}
	base.Commit()
	seq, par := base.Copy(), base.Copy()
	bc := vm.BlockContext{
		Number: 1, Time: 1, GasLimit: 30_000_000, BaseFee: big.NewInt(0),
		Coinbase: crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0"),
		ChainID:  big.NewInt(2026),
	}
	if _, err := vm.NewParallelExecutor(seq, bc, 1).ApplySequential(msgs); err != nil {
		t.Fatal(err)
	}
	pe := vm.NewParallelExecutor(par, bc, 4)
	if _, err := pe.ApplyParallel(msgs); err != nil {
		t.Fatal(err)
	}
	sr, _ := seq.IntermediateRoot()
	pr, _ := par.IntermediateRoot()
	if sr != pr {
		t.Fatal("PE/sequential root mismatch under conflict (consensus safety)")
	}
	if pe.Stats().Rollbacks < 1 {
		t.Fatal("expected rollbacks when one sender serializes all txs")
	}
}

func newTestNode(t *testing.T) *node.Node {
	t.Helper()
	g, err := config.ParseGenesis([]byte(`{
	  "config": {"chainId": 2026, "homesteadBlock": 0, "eip150Block": 0, "eip155Block": 0, "eip158Block": 0,
	    "byzantiumBlock": 0, "constantinopleBlock": 0, "petersburgBlock": 0, "istanbulBlock": 0,
	    "muirGlacierBlock": 0, "berlinBlock": 0, "londonBlock": 0, "shanghaiBlock": 0, "cancunBlock": 0},
	  "timestamp": 0, "extraData": "0x", "gasLimit": "0x7270e00", "baseFeePerGas": "0x3b9aca00",
	  "alloc": {
	    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {"balance": "1000000000000000000000000"}
	  }
	}`))
	if err != nil {
		t.Fatal(err)
	}
	n, err := node.NewFromGenesis(g)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
