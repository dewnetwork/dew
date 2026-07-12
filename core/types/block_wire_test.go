package types_test

import (
	"math/big"
	"testing"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

func TestBlockMarshalRoundTrip_EVMTx(t *testing.T) {
	h := &types.Header{
		ParentHash:  types.Hash{},
		StateRoot:   types.Keccak256Hash([]byte("state")),
		TxRoot:      types.EmptyTxRoot,
		ReceiptRoot: types.EmptyReceiptRoot,
		Number:      1,
		Timestamp:   2,
		GasLimit:    30_000_000,
		GasUsed:     21000,
		BaseFee:     big.NewInt(1_000_000_000),
		Proposer:    crypto.Address{},
	}
	to := crypto.Address{}
	tx := &types.Transaction{
		Type:      types.LegacyTxType,
		Nonce:     0,
		GasFeeCap: big.NewInt(1_000_000_000),
		Gas:       21000,
		To:        &to,
		Value:     big.NewInt(1),
		V:         big.NewInt(27),
		R:         big.NewInt(1),
		S:         big.NewInt(2),
	}
	blk := types.NewBlock(h, []*types.Transaction{tx})
	raw, err := blk.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	out, err := types.UnmarshalBlockBinary(raw)
	if err != nil {
		t.Fatal(err)
	}
	if out.Hash() != blk.Hash() {
		t.Fatalf("hash mismatch: %s vs %s", out.Hash().Hex(), blk.Hash().Hex())
	}
	if len(out.Transactions()) != 1 {
		t.Fatalf("tx count=%d", len(out.Transactions()))
	}
}