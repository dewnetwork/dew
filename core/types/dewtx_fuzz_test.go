package types

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

// FuzzDewTxUnmarshal ensures random bytes never panic and reject closed.
func FuzzDewTxUnmarshal(f *testing.F) {
	// Valid signed corpus seed
	key, err := crypto.GenerateKey()
	if err != nil {
		f.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	tx := NewDewTx(big.NewInt(int64(params.PublicTestnetChainID)), 0, sender, recv, uint256.NewInt(1), params.DefaultDewTxFeeWei, []byte{0x01}, nil)
	if err := SignDewTx(tx, key); err != nil {
		f.Fatal(err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(raw)
	f.Add([]byte{DewTxType})
	f.Add([]byte{0x02, 0xc0})
	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{0xff}, 64))

	f.Fuzz(func(t *testing.T, data []byte) {
		var out DewTx
		_ = out.UnmarshalBinary(data) // must not panic
	})
}

// FuzzDecodeRoundTrip: if unmarshal succeeds, re-marshal prefix must stay DewTxType
// when the input was a well-formed envelope (best-effort).
func FuzzDewTxRoundTripIfValid(f *testing.F) {
	key, _ := crypto.GenerateKey()
	sender := crypto.PubkeyToAddress(&key.PublicKey)
	recv := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	tx := NewDewTx(big.NewInt(2026), 1, sender, recv, uint256.NewInt(42), params.DefaultDewTxFeeWei, nil, []crypto.Address{recv})
	_ = SignDewTx(tx, key)
	raw, _ := tx.MarshalBinary()
	f.Add(raw)

	f.Fuzz(func(t *testing.T, data []byte) {
		var out DewTx
		if err := out.UnmarshalBinary(data); err != nil {
			return
		}
		bin, err := out.MarshalBinary()
		if err != nil {
			t.Fatalf("marshal after successful unmarshal: %v", err)
		}
		if len(bin) == 0 || bin[0] != DewTxType {
			t.Fatalf("bad prefix after round-trip")
		}
	})
}
