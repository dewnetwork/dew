package types

import (
	"fmt"
	"io"
	"math/big"

	"github.com/dewnetwork/dew/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// Transaction type tags (Ethereum wire types).
const (
	LegacyTxType     = byte(0x00)
	AccessListTxType = byte(0x01)
	DynamicFeeTxType = byte(0x02) // EIP-1559 — primary for Phase A
)

// AccessTuple is one EIP-2930 access list entry.
type AccessTuple struct {
	Address     crypto.Address
	StorageKeys []Hash
}

// AccessList is an EIP-2930 access list.
type AccessList []AccessTuple

// Transaction is a Phase A EVM transaction (legacy or EIP-1559).
// Encoding follows Ethereum signed-tx rules for hashing.
type Transaction struct {
	Type         byte // 0x00 legacy, 0x02 dynamic fee
	ChainID      *big.Int
	Nonce        uint64
	GasTipCap    *big.Int // maxPriorityFeePerGas (type 2); gasPrice for legacy
	GasFeeCap    *big.Int // maxFeePerGas (type 2); gasPrice for legacy
	Gas          uint64
	To           *crypto.Address // nil = contract creation
	Value        *big.Int
	Data         []byte
	AccessList   AccessList
	V, R, S      *big.Int
	// cached hash
	hash Hash
	from *crypto.Address
}

// NewDynamicFeeTx builds an unsigned EIP-1559 transaction skeleton.
func NewDynamicFeeTx(
	chainID *big.Int,
	nonce uint64,
	gasTipCap, gasFeeCap *big.Int,
	gas uint64,
	to *crypto.Address,
	value *big.Int,
	data []byte,
) *Transaction {
	return &Transaction{
		Type:      DynamicFeeTxType,
		ChainID:   copyBig(chainID),
		Nonce:     nonce,
		GasTipCap: copyBig(gasTipCap),
		GasFeeCap: copyBig(gasFeeCap),
		Gas:       gas,
		To:        copyAddr(to),
		Value:     copyBig(value),
		Data:      append([]byte(nil), data...),
		V:         new(big.Int),
		R:         new(big.Int),
		S:         new(big.Int),
	}
}

// IsContractCreation reports whether To is nil.
func (tx *Transaction) IsContractCreation() bool {
	return tx.To == nil
}

// Hash returns the Ethereum-compatible transaction hash (Keccak of signed encoding).
func (tx *Transaction) Hash() Hash {
	if tx.hash != (Hash{}) {
		return tx.hash
	}
	enc, err := tx.MarshalBinary()
	if err != nil {
		panic("types: tx encode: " + err.Error())
	}
	tx.hash = Keccak256Hash(enc)
	return tx.hash
}

// UnmarshalBinary decodes a signed transaction wire encoding.
func (tx *Transaction) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("types: empty tx binary")
	}
	if data[0] == DynamicFeeTxType {
		var raw dynamicFeeRLP
		if err := rlp.DecodeBytes(data[1:], &raw); err != nil {
			return err
		}
		*tx = Transaction{
			Type:       DynamicFeeTxType,
			ChainID:    copyBig(raw.ChainID),
			Nonce:      raw.Nonce,
			GasTipCap:  copyBig(raw.GasTipCap),
			GasFeeCap:  copyBig(raw.GasFeeCap),
			Gas:        raw.Gas,
			To:         bytesToAddr(raw.To),
			Value:      copyBig(raw.Value),
			Data:       append([]byte(nil), raw.Data...),
			AccessList: decodeAccessList(raw.AccessList),
			V:          copyBig(raw.YParity),
			R:          copyBig(raw.R),
			S:          copyBig(raw.S),
		}
		return nil
	}
	var raw legacyRLP
	if err := rlp.DecodeBytes(data, &raw); err != nil {
		return err
	}
	gasPrice := copyBig(raw.GasPrice)
	*tx = Transaction{
		Type:      LegacyTxType,
		Nonce:     raw.Nonce,
		GasTipCap: gasPrice,
		GasFeeCap: gasPrice,
		Gas:       raw.Gas,
		To:        bytesToAddr(raw.To),
		Value:     copyBig(raw.Value),
		Data:      append([]byte(nil), raw.Data...),
		V:         copyBig(raw.V),
		R:         copyBig(raw.R),
		S:         copyBig(raw.S),
	}
	return nil
}

// MarshalBinary returns the signed wire encoding (type byte || RLP payload for typed txs).
func (tx *Transaction) MarshalBinary() ([]byte, error) {
	switch tx.Type {
	case LegacyTxType:
		return rlp.EncodeToBytes(tx.legacyRLP())
	case DynamicFeeTxType:
		payload, err := rlp.EncodeToBytes(tx.dynamicFeeRLP())
		if err != nil {
			return nil, err
		}
		return append([]byte{DynamicFeeTxType}, payload...), nil
	default:
		return nil, fmt.Errorf("types: unsupported tx type %d", tx.Type)
	}
}

// EncodeRLP encodes the transaction for inclusion in a block body list.
// For typed txs this stores the typed envelope as a byte string (Ethereum convention).
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	if tx.Type == LegacyTxType {
		return rlp.Encode(w, tx.legacyRLP())
	}
	bin, err := tx.MarshalBinary()
	if err != nil {
		return err
	}
	return rlp.Encode(w, bin)
}

type legacyRLP struct {
	Nonce    uint64
	GasPrice *big.Int
	Gas      uint64
	To       []byte // empty = creation
	Value    *big.Int
	Data     []byte
	V, R, S  *big.Int
}

func (tx *Transaction) legacyRLP() legacyRLP {
	return legacyRLP{
		Nonce:    tx.Nonce,
		GasPrice: copyBig(tx.GasFeeCap),
		Gas:      tx.Gas,
		To:       addrBytes(tx.To),
		Value:    copyBig(tx.Value),
		Data:     tx.Data,
		V:        copyBig(tx.V),
		R:        copyBig(tx.R),
		S:        copyBig(tx.S),
	}
}

// dynamicFeeRLP matches EIP-1559 signed payload (without type prefix).
type dynamicFeeRLP struct {
	ChainID    *big.Int
	Nonce      uint64
	GasTipCap  *big.Int
	GasFeeCap  *big.Int
	Gas        uint64
	To         []byte
	Value      *big.Int
	Data       []byte
	AccessList []accessTupleRLP
	YParity    *big.Int
	R, S       *big.Int
}

type accessTupleRLP struct {
	Address     []byte
	StorageKeys [][]byte
}

func (tx *Transaction) dynamicFeeRLP() dynamicFeeRLP {
	al := make([]accessTupleRLP, len(tx.AccessList))
	for i, t := range tx.AccessList {
		keys := make([][]byte, len(t.StorageKeys))
		for j, k := range t.StorageKeys {
			keys[j] = k.Bytes()
		}
		al[i] = accessTupleRLP{Address: t.Address.Bytes(), StorageKeys: keys}
	}
	return dynamicFeeRLP{
		ChainID:    copyBig(tx.ChainID),
		Nonce:      tx.Nonce,
		GasTipCap:  copyBig(tx.GasTipCap),
		GasFeeCap:  copyBig(tx.GasFeeCap),
		Gas:        tx.Gas,
		To:         addrBytes(tx.To),
		Value:      copyBig(tx.Value),
		Data:       tx.Data,
		AccessList: al,
		YParity:    copyBig(tx.V),
		R:          copyBig(tx.R),
		S:          copyBig(tx.S),
	}
}

// TxRoot returns a Merkle-style root over transaction encodings.
// Phase A: binary-ish root = Keccak of RLP list of each tx's Hash (stable, simple).
// Isolated so a full trie can replace this before testnet freeze.
func TxRoot(txs []*Transaction) Hash {
	if len(txs) == 0 {
		return EmptyTxRoot
	}
	hashes := make([][]byte, len(txs))
	for i, tx := range txs {
		h := tx.Hash()
		hashes[i] = h.Bytes()
	}
	enc, err := rlp.EncodeToBytes(hashes)
	if err != nil {
		panic(err)
	}
	return Keccak256Hash(enc)
}

func copyBig(v *big.Int) *big.Int {
	if v == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(v)
}

func copyAddr(a *crypto.Address) *crypto.Address {
	if a == nil {
		return nil
	}
	c := *a
	return &c
}

func addrBytes(a *crypto.Address) []byte {
	if a == nil {
		return []byte{}
	}
	return a.Bytes()
}

func bytesToAddr(b []byte) *crypto.Address {
	if len(b) == 0 {
		return nil
	}
	if len(b) != 20 {
		return nil
	}
	var a crypto.Address
	copy(a[:], b)
	return &a
}

func decodeAccessList(raw []accessTupleRLP) AccessList {
	if len(raw) == 0 {
		return nil
	}
	out := make(AccessList, len(raw))
	for i, t := range raw {
		keys := make([]Hash, len(t.StorageKeys))
		for j, k := range t.StorageKeys {
			keys[j] = BytesToHash(k)
		}
		addr := bytesToAddr(t.Address)
		var address crypto.Address
		if addr != nil {
			address = *addr
		}
		out[i] = AccessTuple{
			Address:     address,
			StorageKeys: keys,
		}
	}
	return out
}
