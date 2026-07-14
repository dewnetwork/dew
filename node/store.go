package node

import (
	"encoding/binary"
	"fmt"
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"

	dewtypes "github.com/dewnetwork/dew/core/types"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

// Chaindata schema version stored at meta/version.
const chainSchemaVersion uint32 = 1

// Key prefixes (must not collide with state a/s/c).
const (
	prefixHeader    byte = 'H'
	prefixBody      byte = 'B'
	prefixCanonical byte = 'N'
	prefixReceipt   byte = 'R'
	prefixTxLookup  byte = 'T'
	// prefixLogIndex secondary keys for O(range) eth_getLogs:
	// L | blockNum u64 BE | txIndex u32 BE | logIndex u32 BE → RLP log payload.
	prefixLogIndex byte = 'L'
)

// logIndexSchemaVersion is the secondary log-index layout (independent of chainSchemaVersion).
const logIndexSchemaVersion uint32 = 1

var (
	metaVersionKey         = []byte("meta/version")
	metaChainIDKey         = []byte("meta/chainId")
	metaGenesisHashKey     = []byte("meta/genesisHash")
	metaTipKey             = []byte("meta/tip")
	metaLogIndexVersionKey = []byte("meta/logIndexVersion")
)

const (
	txKindEVM byte = 0
	txKindDew byte = 1
)

func headerKey(hash dewtypes.Hash) []byte {
	k := make([]byte, 1+32)
	k[0] = prefixHeader
	copy(k[1:], hash[:])
	return k
}

func bodyKey(hash dewtypes.Hash) []byte {
	k := make([]byte, 1+32)
	k[0] = prefixBody
	copy(k[1:], hash[:])
	return k
}

func canonicalKey(num uint64) []byte {
	k := make([]byte, 1+8)
	k[0] = prefixCanonical
	binary.BigEndian.PutUint64(k[1:], num)
	return k
}

func receiptKey(txHash dewtypes.Hash) []byte {
	k := make([]byte, 1+32)
	k[0] = prefixReceipt
	copy(k[1:], txHash[:])
	return k
}

func txLookupKey(txHash dewtypes.Hash) []byte {
	k := make([]byte, 1+32)
	k[0] = prefixTxLookup
	copy(k[1:], txHash[:])
	return k
}

// logIndexKey is ordered by block number then tx/log index for range scans.
func logIndexKey(blockNum uint64, txIndex, logIndex uint) []byte {
	k := make([]byte, 1+8+4+4)
	k[0] = prefixLogIndex
	binary.BigEndian.PutUint64(k[1:], blockNum)
	binary.BigEndian.PutUint32(k[9:], uint32(txIndex))
	binary.BigEndian.PutUint32(k[13:], uint32(logIndex))
	return k
}

// logIndexBlockPrefix matches all log-index keys for one block height.
func logIndexBlockPrefix(blockNum uint64) []byte {
	k := make([]byte, 1+8)
	k[0] = prefixLogIndex
	binary.BigEndian.PutUint64(k[1:], blockNum)
	return k
}

func parseLogIndexKey(key []byte) (blockNum uint64, txIndex, logIndex uint, ok bool) {
	if len(key) != 1+8+4+4 || key[0] != prefixLogIndex {
		return 0, 0, 0, false
	}
	blockNum = binary.BigEndian.Uint64(key[1:9])
	txIndex = uint(binary.BigEndian.Uint32(key[9:13]))
	logIndex = uint(binary.BigEndian.Uint32(key[13:17]))
	return blockNum, txIndex, logIndex, true
}

type logIndexStorage struct {
	Address   []byte
	Topics    [][]byte
	Data      []byte
	TxHash    []byte
	BlockHash []byte
}

func encodeLogIndex(lg *dewtypes.Log, txHash, blockHash dewtypes.Hash) ([]byte, error) {
	if lg == nil {
		return nil, fmt.Errorf("node: nil log")
	}
	st := logIndexStorage{
		Address:   lg.Address.Bytes(),
		Data:      lg.Data,
		TxHash:    txHash.Bytes(),
		BlockHash: blockHash.Bytes(),
	}
	st.Topics = make([][]byte, len(lg.Topics))
	for i, t := range lg.Topics {
		st.Topics[i] = t.Bytes()
	}
	return rlp.EncodeToBytes(st)
}

func decodeLogIndex(blob []byte) (*dewtypes.Log, dewtypes.Hash, dewtypes.Hash, error) {
	var st logIndexStorage
	if err := rlp.DecodeBytes(blob, &st); err != nil {
		return nil, dewtypes.Hash{}, dewtypes.Hash{}, err
	}
	var addr dewcrypto.Address
	copy(addr[:], st.Address)
	topics := make([]dewtypes.Hash, len(st.Topics))
	for i, t := range st.Topics {
		topics[i] = dewtypes.BytesToHash(t)
	}
	lg := &dewtypes.Log{
		Address: addr,
		Topics:  topics,
		Data:    st.Data,
	}
	return lg, dewtypes.BytesToHash(st.TxHash), dewtypes.BytesToHash(st.BlockHash), nil
}

func putLogIndexVersion(w putWriter, ver uint32) error {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, ver)
	return w.Put(metaLogIndexVersionKey, b)
}

func readLogIndexVersion(database db.Database) (uint32, bool, error) {
	blob, err := database.Get(metaLogIndexVersionKey)
	if err == db.ErrNotFound {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if len(blob) < 4 {
		return 0, false, fmt.Errorf("node: invalid meta/logIndexVersion")
	}
	return binary.BigEndian.Uint32(blob[:4]), true, nil
}

func encodeTip(height uint64, hash dewtypes.Hash) ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{height, hash.Bytes()})
}

func decodeTip(blob []byte) (uint64, dewtypes.Hash, error) {
	var raw struct {
		Height uint64
		Hash   []byte
	}
	if err := rlp.DecodeBytes(blob, &raw); err != nil {
		return 0, dewtypes.Hash{}, err
	}
	return raw.Height, dewtypes.BytesToHash(raw.Hash), nil
}

func encodeHeader(h *dewtypes.Header) ([]byte, error) {
	return rlp.EncodeToBytes(h)
}

func decodeHeader(blob []byte) (*dewtypes.Header, error) {
	h := new(dewtypes.Header)
	if err := rlp.DecodeBytes(blob, h); err != nil {
		return nil, err
	}
	return h, nil
}

func encodeBody(txs []*dewtypes.Transaction) ([]byte, error) {
	raws := make([][]byte, 0, len(txs))
	for _, tx := range txs {
		if tx == nil {
			continue
		}
		bin, err := tx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		raws = append(raws, bin)
	}
	return rlp.EncodeToBytes(raws)
}

func decodeBody(blob []byte) ([]*dewtypes.Transaction, error) {
	var raws [][]byte
	if err := rlp.DecodeBytes(blob, &raws); err != nil {
		return nil, err
	}
	txs := make([]*dewtypes.Transaction, 0, len(raws))
	for _, raw := range raws {
		tx := new(dewtypes.Transaction)
		if err := tx.UnmarshalBinary(raw); err != nil {
			return nil, err
		}
		txs = append(txs, tx)
	}
	return txs, nil
}

type receiptStorage struct {
	Type              byte
	Status            uint64
	CumulativeGasUsed uint64
	GasUsed           uint64
	EffectiveGasPrice *big.Int
	Logs              []logStorage
	ContractAddress   []byte
	TxHash            []byte
	BlockHash         []byte
	BlockNumber       uint64
	TransactionIndex  uint
}

type logStorage struct {
	Address []byte
	Topics  [][]byte
	Data    []byte
}

func encodeReceipt(r *dewtypes.Receipt) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("node: nil receipt")
	}
	st := receiptStorage{
		Type:              r.Type,
		Status:            r.Status,
		CumulativeGasUsed: r.CumulativeGasUsed,
		GasUsed:           r.GasUsed,
		EffectiveGasPrice: r.EffectiveGasPrice,
		TxHash:            r.TxHash.Bytes(),
		BlockHash:         r.BlockHash.Bytes(),
		BlockNumber:       r.BlockNumber,
		TransactionIndex:  r.TransactionIndex,
	}
	if r.ContractAddress != nil {
		st.ContractAddress = r.ContractAddress.Bytes()
	}
	for _, lg := range r.Logs {
		if lg == nil {
			continue
		}
		topics := make([][]byte, len(lg.Topics))
		for i, t := range lg.Topics {
			topics[i] = t.Bytes()
		}
		st.Logs = append(st.Logs, logStorage{
			Address: lg.Address.Bytes(),
			Topics:  topics,
			Data:    lg.Data,
		})
	}
	return rlp.EncodeToBytes(st)
}

func decodeReceipt(blob []byte) (*dewtypes.Receipt, error) {
	var st receiptStorage
	if err := rlp.DecodeBytes(blob, &st); err != nil {
		return nil, err
	}
	r := &dewtypes.Receipt{
		Type:              st.Type,
		Status:            st.Status,
		CumulativeGasUsed: st.CumulativeGasUsed,
		GasUsed:           st.GasUsed,
		EffectiveGasPrice: st.EffectiveGasPrice,
		TxHash:            dewtypes.BytesToHash(st.TxHash),
		BlockHash:         dewtypes.BytesToHash(st.BlockHash),
		BlockNumber:       st.BlockNumber,
		TransactionIndex:  st.TransactionIndex,
	}
	if len(st.ContractAddress) == 20 {
		var a dewcrypto.Address
		copy(a[:], st.ContractAddress)
		r.ContractAddress = &a
	}
	for _, ls := range st.Logs {
		var addr dewcrypto.Address
		copy(addr[:], ls.Address)
		topics := make([]dewtypes.Hash, len(ls.Topics))
		for i, t := range ls.Topics {
			topics[i] = dewtypes.BytesToHash(t)
		}
		r.Logs = append(r.Logs, &dewtypes.Log{
			Address: addr,
			Topics:  topics,
			Data:    ls.Data,
		})
	}
	return r, nil
}

type txLookupStorage struct {
	BlockHash   []byte
	BlockNumber uint64
	Index       uint
	From        []byte
	Kind        byte
	Raw         []byte
}

func encodeTxLookup(look *TxLookup) ([]byte, error) {
	if look == nil {
		return nil, fmt.Errorf("node: nil tx lookup")
	}
	st := txLookupStorage{
		BlockHash:   look.BlockHash.Bytes(),
		BlockNumber: look.BlockNumber,
		Index:       look.Index,
		From:        look.From.Bytes(),
	}
	switch {
	case look.DewTx != nil:
		st.Kind = txKindDew
		raw, err := look.DewTx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		st.Raw = raw
	case look.Tx != nil:
		st.Kind = txKindEVM
		raw, err := look.Tx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		st.Raw = raw
	default:
		return nil, fmt.Errorf("node: tx lookup missing payload")
	}
	return rlp.EncodeToBytes(st)
}

func decodeTxLookup(blob []byte, txHash dewtypes.Hash) (*TxLookup, error) {
	var st txLookupStorage
	if err := rlp.DecodeBytes(blob, &st); err != nil {
		return nil, err
	}
	look := &TxLookup{
		BlockHash:   dewtypes.BytesToHash(st.BlockHash),
		BlockNumber: st.BlockNumber,
		Index:       st.Index,
		TxHash:      txHash,
	}
	copy(look.From[:], st.From)
	switch st.Kind {
	case txKindDew:
		tx := new(dewtypes.DewTx)
		if err := tx.UnmarshalBinary(st.Raw); err != nil {
			return nil, err
		}
		look.DewTx = tx
	case txKindEVM:
		tx := new(ethtypes.Transaction)
		if err := tx.UnmarshalBinary(st.Raw); err != nil {
			return nil, err
		}
		look.Tx = tx
	default:
		return nil, fmt.Errorf("node: unknown tx kind %d", st.Kind)
	}
	return look, nil
}

func putMeta(w putWriter, chainID *big.Int, genesisHash dewtypes.Hash) error {
	ver := make([]byte, 4)
	binary.BigEndian.PutUint32(ver, chainSchemaVersion)
	if err := w.Put(metaVersionKey, ver); err != nil {
		return err
	}
	cid, err := rlp.EncodeToBytes(chainID)
	if err != nil {
		return err
	}
	if err := w.Put(metaChainIDKey, cid); err != nil {
		return err
	}
	return w.Put(metaGenesisHashKey, genesisHash.Bytes())
}

func readMetaVersion(database db.Database) (uint32, bool, error) {
	blob, err := database.Get(metaVersionKey)
	if err == db.ErrNotFound {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if len(blob) < 4 {
		return 0, false, fmt.Errorf("node: invalid meta/version")
	}
	return binary.BigEndian.Uint32(blob[:4]), true, nil
}

func readMetaGenesisHash(database db.Database) (dewtypes.Hash, error) {
	blob, err := database.Get(metaGenesisHashKey)
	if err != nil {
		return dewtypes.Hash{}, err
	}
	return dewtypes.BytesToHash(blob), nil
}

func readMetaChainID(database db.Database) (*big.Int, error) {
	blob, err := database.Get(metaChainIDKey)
	if err != nil {
		return nil, err
	}
	var id big.Int
	if err := rlp.DecodeBytes(blob, &id); err != nil {
		return nil, err
	}
	return &id, nil
}

func readTip(database db.Database) (uint64, dewtypes.Hash, error) {
	blob, err := database.Get(metaTipKey)
	if err != nil {
		return 0, dewtypes.Hash{}, err
	}
	return decodeTip(blob)
}

type putWriter interface {
	Put(key, value []byte) error
}
