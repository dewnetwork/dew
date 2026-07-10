package native

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/params"
)

func setup(t *testing.T) (*state.StateDB, *ecdsa.PrivateKey, crypto.Address, crypto.Address, crypto.Address) {
	t.Helper()
	mdb := db.NewMemoryDB()
	t.Cleanup(func() { mdb.Close() })
	s := state.New(mdb)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	sink := crypto.MustHexToAddress("0x00000000000000000000000000000000000000c0")
	s.SetBalance(sender, uint256.NewInt(params.DefaultDewTxFeeWei*10+10_000))
	return s, key, sender, recv, sink
}

func TestNative_Transfer(t *testing.T) {
	s, key, sender, recv, sink := setup(t)

	tx := types.NewDewTx(big.NewInt(2026), 0, sender, recv, uint256.NewInt(1000), params.DefaultDewTxFeeWei, nil, nil)
	if err := types.SignDewTx(tx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.RecoverSender(); err != nil {
		t.Fatal(err)
	}

	exec := NewExecutor(s, sink)
	res, err := exec.ApplyDewTx(tx)
	if err != nil || res.Failed {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if s.GetBalance(recv).Uint64() != 1000 {
		t.Fatalf("recv bal %s", s.GetBalance(recv))
	}
	if s.GetNonce(sender) != 1 {
		t.Fatalf("nonce %d", s.GetNonce(sender))
	}
	if s.GetBalance(sink).Uint64() != params.DefaultDewTxFeeWei {
		t.Fatalf("fee sink %s", s.GetBalance(sink))
	}
}

func TestNative_FailClosedIncompleteAccessList(t *testing.T) {
	s, key, sender, recv, sink := setup(t)
	third := crypto.MustHexToAddress("0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC")

	// Credit third party without listing them in AccessList
	payload := make([]byte, 1+20+32)
	payload[0] = PayloadCredit
	copy(payload[1:21], third[:])
	b32 := uint256.NewInt(50).Bytes32()
	copy(payload[21:], b32[:])

	tx := types.NewDewTx(big.NewInt(2026), 0, sender, recv, uint256.NewInt(0), params.DefaultDewTxFeeWei, payload, nil)
	if err := types.SignDewTx(tx, key); err != nil {
		t.Fatal(err)
	}
	exec := NewExecutor(s, sink)
	res, err := exec.ApplyDewTx(tx)
	if err != nil {
		t.Fatalf("outer err: %v", err)
	}
	if !res.Failed || res.Err == nil {
		t.Fatal("expected fail-closed incomplete access list")
	}
	if s.GetNonce(sender) != 0 {
		t.Fatal("nonce should not advance on failed native tx")
	}
	if !s.GetBalance(third).IsZero() {
		t.Fatal("third should not be credited")
	}
}

func TestNative_CreditWithAccessList(t *testing.T) {
	s, key, sender, recv, sink := setup(t)
	third := crypto.MustHexToAddress("0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC")

	payload := make([]byte, 1+20+32)
	payload[0] = PayloadCredit
	copy(payload[1:21], third[:])
	b32 := uint256.NewInt(50).Bytes32()
	copy(payload[21:], b32[:])

	tx := types.NewDewTx(big.NewInt(2026), 0, sender, recv, uint256.NewInt(10), params.DefaultDewTxFeeWei, payload, []crypto.Address{third})
	if err := types.SignDewTx(tx, key); err != nil {
		t.Fatal(err)
	}
	exec := NewExecutor(s, sink)
	res, err := exec.ApplyDewTx(tx)
	if err != nil || res.Failed {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if s.GetBalance(recv).Uint64() != 10 {
		t.Fatalf("recv %s", s.GetBalance(recv))
	}
	if s.GetBalance(third).Uint64() != 50 {
		t.Fatalf("third %s", s.GetBalance(third))
	}
}
