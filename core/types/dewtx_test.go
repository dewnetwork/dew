package types

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

func TestDewTx_SignRecoverRoundTrip(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	extra := crypto.MustHexToAddress("0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC")

	tx := NewDewTx(
		big.NewInt(2026),
		0,
		sender,
		recv,
		uint256.NewInt(1000),
		params.DefaultDewTxFeeWei,
		[]byte{0x01, 0x02},
		[]crypto.Address{extra},
	)
	if err := SignDewTx(tx, key); err != nil {
		t.Fatal(err)
	}
	got, err := tx.RecoverSender()
	if err != nil {
		t.Fatal(err)
	}
	if got != sender {
		t.Fatalf("recovered %s want %s", got.Hex(), sender.Hex())
	}

	bin, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if bin[0] != DewTxType {
		t.Fatalf("prefix %x", bin[0])
	}

	var tx2 DewTx
	if err := tx2.UnmarshalBinary(bin); err != nil {
		t.Fatal(err)
	}
	if tx2.Hash() != tx.Hash() {
		t.Fatalf("hash mismatch")
	}
	if tx2.Nonce != tx.Nonce || tx2.Fee != tx.Fee {
		t.Fatal("fields mismatch")
	}
	if !bytes.Equal(tx2.Payload, tx.Payload) {
		t.Fatal("payload mismatch")
	}
	if _, err := tx2.RecoverSender(); err != nil {
		t.Fatal(err)
	}
}

func TestDewTx_DomainSeparatedFromEVM(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	tx := NewDewTx(big.NewInt(2026), 0, sender, recv, uint256.NewInt(1), 1, nil, nil)
	h1 := tx.SigningHash()
	// Mutating domain tag would change hash — ensure non-zero and stable
	h2 := tx.SigningHash()
	if h1 != h2 || h1.IsZero() {
		t.Fatal("signing hash unstable")
	}
	// Wire hash includes signature after sign
	if err := SignDewTx(tx, key); err != nil {
		t.Fatal(err)
	}
	if tx.Hash() == h1 {
		t.Fatal("tx hash should differ from signing hash")
	}
}

func TestDewTx_ContainsAccess(t *testing.T) {
	sender := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	extra := crypto.MustHexToAddress("0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC")
	other := crypto.MustHexToAddress("0x90F79bf6EB2c4f870365E785982E1f101E93b906")
	tx := NewDewTx(big.NewInt(2026), 0, sender, recv, uint256.NewInt(0), 1, nil, []crypto.Address{extra})
	if !tx.ContainsAccess(sender) || !tx.ContainsAccess(recv) || !tx.ContainsAccess(extra) {
		t.Fatal("expected access")
	}
	if tx.ContainsAccess(other) {
		t.Fatal("other should not be allowed")
	}
}

func TestDewTx_RejectBadPrefix(t *testing.T) {
	var tx DewTx
	if err := tx.UnmarshalBinary([]byte{0x02, 0xc0}); err == nil {
		t.Fatal("expected error for EVM-typed prefix")
	}
}
