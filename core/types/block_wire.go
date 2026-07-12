package types

import (
	"fmt"

	"github.com/ethereum/go-ethereum/rlp"
)

// MarshalBinary encodes header + raw tx payloads for P2P/sync.
// Wire: RLP([headerRLP bytes, [txRaw0, txRaw1, ...]])
func (b *Block) MarshalBinary() ([]byte, error) {
	if b == nil || b.header == nil {
		return nil, fmt.Errorf("types: nil block")
	}
	hdr, err := rlp.EncodeToBytes(b.header)
	if err != nil {
		return nil, err
	}
	raws := make([][]byte, 0, len(b.transactions))
	for _, tx := range b.transactions {
		if tx == nil {
			continue
		}
		bin, err := tx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		raws = append(raws, bin)
	}
	return rlp.EncodeToBytes([]interface{}{hdr, raws})
}

// UnmarshalBlockBinary decodes a wire block.
func UnmarshalBlockBinary(data []byte) (*Block, error) {
	var payload struct {
		Header []byte
		TxRaws [][]byte
	}
	if err := rlp.DecodeBytes(data, &payload); err != nil {
		return nil, err
	}
	h := new(Header)
	if err := rlp.DecodeBytes(payload.Header, h); err != nil {
		return nil, err
	}
	txs := make([]*Transaction, 0, len(payload.TxRaws))
	for _, raw := range payload.TxRaws {
		tx := new(Transaction)
		if err := tx.UnmarshalBinary(raw); err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return NewBlock(h, txs), nil
}