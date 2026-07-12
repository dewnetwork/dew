// Package node is the in-process Dew full-node backend used by JSON-RPC (Phase A4+).
// Dev mode auto-seals one block per accepted transaction.
package node

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/core/state"
	dewtypes "github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/core/vm"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/mempool"
	"github.com/dewnetwork/dew/params"
)

// Node holds chain state and applies transactions sequentially.
type Node struct {
	mu sync.RWMutex

	genesis  *config.Genesis
	chainID  *big.Int
	db       db.Database
	statedb  *state.StateDB
	header   *dewtypes.Header

	blocks   map[dewtypes.Hash]*dewtypes.Block
	blockNum map[uint64]dewtypes.Hash
	txIndex  map[dewtypes.Hash]*TxLookup
	receipts map[dewtypes.Hash]*dewtypes.Receipt

	// logs index (simple linear scan for eth_getLogs)
	allLogs []*IndexedLog

	gasPrice *big.Int
	baseFee  *big.Int

	// Phase B feature flags / metrics
	enableNative      bool
	enablePrecompiles bool
	enableStaking     bool // Phase C4: live 0x102 (default off)
	peStats           vm.ExecutionStats

	// Phase C1: unified mempool admission (EVM + DewTx)
	pool *mempool.Pool

	// Dev auto-mine: seal one block per accepted tx (default true).
	autoMine bool
}

// TxLookup links a transaction hash to its block placement.
type TxLookup struct {
	BlockHash   dewtypes.Hash
	BlockNumber uint64
	Index       uint
	Tx          *ethtypes.Transaction // EVM tx; nil for DewTx
	DewTx       *dewtypes.DewTx       // native tx; nil for EVM
	From        dewcrypto.Address
	TxHash      dewtypes.Hash
}

// IndexedLog is a log with block/tx metadata for eth_getLogs.
type IndexedLog struct {
	Log         *dewtypes.Log
	BlockNumber uint64
	BlockHash   dewtypes.Hash
	TxHash      dewtypes.Hash
	TxIndex     uint
	Index       uint
}

// NewFromGenesis commits genesis into a fresh MemoryDB-backed node.
func NewFromGenesis(g *config.Genesis) (*Node, error) {
	mdb := db.NewMemoryDB()
	block, statedb, err := g.Commit(mdb)
	if err != nil {
		return nil, err
	}
	h := block.Header()
	n := &Node{
		genesis:           g,
		chainID:           g.ChainID(),
		db:                mdb,
		statedb:           statedb,
		header:            h,
		blocks:            map[dewtypes.Hash]*dewtypes.Block{block.Hash(): block},
		blockNum:          map[uint64]dewtypes.Hash{0: block.Hash()},
		txIndex:           make(map[dewtypes.Hash]*TxLookup),
		receipts:          make(map[dewtypes.Hash]*dewtypes.Receipt),
		gasPrice:          big.NewInt(1_000_000_000), // 1 gwei
		baseFee:           new(big.Int).Set(h.BaseFee),
		enableNative:      params.DefaultEnableNativePath,
		enablePrecompiles: params.DefaultEnableDewPrecompiles,
		enableStaking:     params.DefaultEnableStaking,
		pool:              mempool.New(mempool.DefaultConfig()),
		autoMine:          true,
	}
	if n.baseFee == nil {
		n.baseFee = big.NewInt(1_000_000_000)
	}
	// Align pool gas floor with node gas price suggestion when higher than default.
	if n.gasPrice != nil && n.gasPrice.Cmp(n.pool.Config().MinGasPriceWei) > 0 {
		cfg := mempool.DefaultConfig()
		cfg.MinGasPriceWei = new(big.Int).Set(n.gasPrice)
		n.pool = mempool.New(cfg)
	}
	return n, nil
}

// SetMempoolConfig replaces the admission pool (e.g. tests / operator tuning).
func (n *Node) SetMempoolConfig(cfg mempool.Config) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.pool = mempool.New(cfg)
}

// Mempool returns the shared admission pool (EVM + DewTx).
func (n *Node) Mempool() *mempool.Pool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.pool
}

// MempoolStats returns (pending count, unique senders).
func (n *Node) MempoolStats() (global, senders int) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.pool.Stats()
}

// SetNativeEnabled toggles dew_sendRawTransaction / DewTx execution.
func (n *Node) SetNativeEnabled(v bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enableNative = v
}

// NativeEnabled reports whether the Dew-native path is on.
func (n *Node) NativeEnabled() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.enableNative
}

// SetPrecompilesEnabled toggles Dew system precompiles (0x100+).
func (n *Node) SetPrecompilesEnabled(v bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enablePrecompiles = v
}

// PrecompilesEnabled reports whether Dew precompiles are active.
func (n *Node) PrecompilesEnabled() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.enablePrecompiles
}

// SetStakingEnabled toggles live staking methods on 0x102 (C4).
func (n *Node) SetStakingEnabled(v bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enableStaking = v
}

// StakingEnabled reports whether 0x102 staking methods are live.
func (n *Node) StakingEnabled() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.enableStaking
}

// SetAutoMine toggles dev per-tx sealing (default true in NewFromGenesis).
func (n *Node) SetAutoMine(v bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.autoMine = v
}

// AutoMine reports whether accepted txs are executed and sealed locally.
func (n *Node) AutoMine() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.autoMine
}

// ExecutionStats returns Dew-PE / operational metrics.
func (n *Node) ExecutionStats() vm.ExecutionStats {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.peStats
}

// RecordExecutionStats stores the latest PE metrics (for multi-tx block paths).
func (n *Node) RecordExecutionStats(st vm.ExecutionStats) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.peStats = st
}

// ChainID returns the network chain id.
func (n *Node) ChainID() *big.Int {
	return new(big.Int).Set(n.chainID)
}

// BlockNumber returns the latest height.
func (n *Node) BlockNumber() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.header.Number
}

// CurrentHeader returns a copy of the head header.
func (n *Node) CurrentHeader() *dewtypes.Header {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.header.Copy()
}

// GetBlockByNumber returns the block at height (nil if missing).
func (n *Node) GetBlockByNumber(num uint64) *dewtypes.Block {
	n.mu.RLock()
	defer n.mu.RUnlock()
	h, ok := n.blockNum[num]
	if !ok {
		return nil
	}
	return n.blocks[h]
}

// GetBlockByHash returns the block with the given hash.
func (n *Node) GetBlockByHash(hash dewtypes.Hash) *dewtypes.Block {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.blocks[hash]
}

// GetBalance returns the balance at latest state.
func (n *Node) GetBalance(addr dewcrypto.Address) *uint256.Int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.statedb.GetBalance(addr)
}

// GetNonce returns the account nonce at latest state.
func (n *Node) GetNonce(addr dewcrypto.Address) uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.statedb.GetNonce(addr)
}

// GetCode returns contract code at latest state.
func (n *Node) GetCode(addr dewcrypto.Address) []byte {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.statedb.GetCode(addr)
}

// GetStorageAt returns a storage slot at latest state.
func (n *Node) GetStorageAt(addr dewcrypto.Address, slot dewtypes.Hash) dewtypes.Hash {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.statedb.GetState(addr, slot)
}

// GasPrice returns the suggested gas price.
func (n *Node) GasPrice() *big.Int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return new(big.Int).Set(n.gasPrice)
}

// BaseFee returns the current base fee.
func (n *Node) BaseFee() *big.Int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return new(big.Int).Set(n.baseFee)
}

// GetTransaction returns indexed tx metadata.
func (n *Node) GetTransaction(hash dewtypes.Hash) *TxLookup {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.txIndex[hash]
}

// GetReceipt returns a receipt by tx hash.
func (n *Node) GetReceipt(hash dewtypes.Hash) *dewtypes.Receipt {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.receipts[hash]
}

// SendDewRawTransaction decodes a signed DewTx, admits via mempool, executes, and auto-mines.
func (n *Node) SendDewRawTransaction(raw []byte) (dewtypes.Hash, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.enableNative {
		return dewtypes.Hash{}, fmt.Errorf("native path disabled")
	}
	tx := new(dewtypes.DewTx)
	if err := tx.UnmarshalBinary(raw); err != nil {
		return dewtypes.Hash{}, fmt.Errorf("invalid DewTx: %w", err)
	}
	if tx.ChainID == nil || tx.ChainID.Cmp(n.chainID) != 0 {
		return dewtypes.Hash{}, fmt.Errorf("wrong chain id: got %v want %s", tx.ChainID, n.chainID)
	}
	if _, err := tx.RecoverSender(); err != nil {
		return dewtypes.Hash{}, fmt.Errorf("invalid signature: %w", err)
	}

	// C1 admission (shared surface with EVM): size / fee / pool limits.
	hash, err := n.pool.AddDew(tx, raw, n.chainID)
	if err != nil {
		return dewtypes.Hash{}, err
	}
	if !n.autoMine {
		return hash, nil
	}
	// Dev auto-mine path: drop from pool once we attempt inclusion.
	defer n.pool.Remove(hash)

	feeSink := n.header.Proposer
	exec := native.NewExecutor(n.statedb, feeSink)
	result, err := exec.ApplyDewTx(tx)
	if err != nil {
		return dewtypes.Hash{}, err
	}
	if result.Failed {
		// Fail-closed incomplete access list: do not mine a success block;
		// surface as RPC error (tx not included).
		return dewtypes.Hash{}, result.Err
	}

	parent := n.header.Copy()
	newHeader := &dewtypes.Header{
		ParentHash:  parent.Hash(),
		Number:      parent.Number + 1,
		Timestamp:   uint64(time.Now().Unix()),
		GasLimit:    parent.GasLimit,
		GasUsed:     0, // flat fee path — no EVM gas
		BaseFee:     new(big.Int).Set(n.baseFee),
		ExtraData:   parent.ExtraData,
		Proposer:    parent.Proposer,
		TxRoot:      dewtypes.EmptyTxRoot,
		ReceiptRoot: dewtypes.EmptyReceiptRoot,
	}
	if newHeader.Timestamp <= parent.Timestamp {
		newHeader.Timestamp = parent.Timestamp + 1
	}

	txHash := tx.Hash()
	root, err := n.statedb.Commit()
	if err != nil {
		return dewtypes.Hash{}, err
	}
	newHeader.StateRoot = root
	newHeader.TxRoot = dewtypes.Keccak256Hash(txHash.Bytes())

	block := dewtypes.NewBlock(newHeader, nil)
	blockHash := block.Hash()

	receipt := &dewtypes.Receipt{
		Type:              dewtypes.DewTxType,
		Status:            1,
		CumulativeGasUsed: 0,
		GasUsed:           0,
		EffectiveGasPrice: big.NewInt(0),
		Logs:              nil,
		TxHash:            txHash,
		BlockHash:         blockHash,
		BlockNumber:       newHeader.Number,
		TransactionIndex:  0,
	}

	n.blocks[blockHash] = block
	n.blockNum[newHeader.Number] = blockHash
	n.header = newHeader
	n.txIndex[txHash] = &TxLookup{
		BlockHash:   blockHash,
		BlockNumber: newHeader.Number,
		Index:       0,
		DewTx:       tx,
		From:        tx.Sender,
		TxHash:      txHash,
	}
	n.receipts[txHash] = receipt
	return txHash, nil
}

// SendRawTransaction decodes, admits via mempool, validates, executes, and auto-mines.
func (n *Node) SendRawTransaction(raw []byte) (dewtypes.Hash, error) {
	tx := new(ethtypes.Transaction)
	if err := tx.UnmarshalBinary(raw); err != nil {
		return dewtypes.Hash{}, fmt.Errorf("invalid transaction: %w", err)
	}
	signer := ethtypes.LatestSignerForChainID(n.chainID)
	fromEth, err := ethtypes.Sender(signer, tx)
	if err != nil {
		return dewtypes.Hash{}, fmt.Errorf("invalid signature: %w", err)
	}
	from := ethToDewAddr(fromEth)

	n.mu.Lock()
	defer n.mu.Unlock()

	// Basic checks
	if tx.ChainId() != nil && tx.ChainId().Sign() != 0 && tx.ChainId().Cmp(n.chainID) != 0 {
		return dewtypes.Hash{}, fmt.Errorf("wrong chain id: got %s want %s", tx.ChainId(), n.chainID)
	}
	nonce := n.statedb.GetNonce(from)
	if tx.Nonce() != nonce {
		return dewtypes.Hash{}, fmt.Errorf("nonce too low/high: got %d want %d", tx.Nonce(), nonce)
	}

	// C1 admission (shared surface with DewTx): size / fee / pool limits / RBF.
	if _, err := n.pool.AddEVM(tx, from, raw, n.chainID); err != nil {
		return dewtypes.Hash{}, err
	}
	txHash := dewtypes.BytesToHash(tx.Hash().Bytes())
	if !n.autoMine {
		return txHash, nil
	}
	defer n.pool.Remove(txHash)

	parent := n.header.Copy()
	newHeader := &dewtypes.Header{
		ParentHash:  parent.Hash(),
		Number:      parent.Number + 1,
		Timestamp:   uint64(time.Now().Unix()),
		GasLimit:    parent.GasLimit,
		GasUsed:     0,
		BaseFee:     new(big.Int).Set(n.baseFee),
		ExtraData:   parent.ExtraData,
		Proposer:    parent.Proposer,
		TxRoot:      dewtypes.EmptyTxRoot,
		ReceiptRoot: dewtypes.EmptyReceiptRoot,
	}
	if newHeader.Timestamp <= parent.Timestamp {
		newHeader.Timestamp = parent.Timestamp + 1
	}

	dewTx, err := ethTxToDew(tx)
	if err != nil {
		return dewtypes.Hash{}, err
	}
	execRes, gasUsed, err := n.executeEVMTxLocked(newHeader, dewTx, 0, 0)
	if err != nil {
		return dewtypes.Hash{}, err
	}
	newHeader.GasUsed = gasUsed

	root, err := n.statedb.Commit()
	if err != nil {
		return dewtypes.Hash{}, err
	}
	newHeader.StateRoot = root
	newHeader.TxRoot = dewtypes.Keccak256Hash(txHash.Bytes())

	block := dewtypes.NewBlock(newHeader, []*dewtypes.Transaction{dewTx})
	n.commitBlockLocked(block, []txExecResult{execRes})

	return txHash, nil
}

// TransactionsInBlock returns tx lookups for a block number.
func (n *Node) TransactionsInBlock(num uint64) []*TxLookup {
	n.mu.RLock()
	defer n.mu.RUnlock()
	var out []*TxLookup
	for _, look := range n.txIndex {
		if look.BlockNumber == num {
			out = append(out, look)
		}
	}
	return out
}

// Call executes eth_call against latest state without committing.
func (n *Node) Call(msg vm.Message) ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Work on a throwaway statedb snapshot by reusing journal Snapshot
	snap := n.statedb.Snapshot()
	defer n.statedb.RevertToSnapshot(snap)

	if msg.GasLimit == 0 {
		msg.GasLimit = 30_000_000
	}
	if msg.GasPrice == nil {
		msg.GasPrice = big.NewInt(0)
	}
	if msg.Value == nil {
		msg.Value = uint256.NewInt(0)
	}

	// Fund gas virtually: if gas price > 0, ensure balance; for eth_call use free gas
	msg.GasPrice = big.NewInt(0)
	msg.NoFinalise = true

	exec := vm.NewExecutor(n.statedb, vm.BlockContext{
		Number:   n.header.Number,
		Time:     n.header.Timestamp,
		GasLimit: n.header.GasLimit,
		BaseFee:  n.baseFee,
		Coinbase: n.header.Proposer,
		ChainID:  n.chainID,
	})
	res, err := exec.ApplyMessage(msg)
	if err != nil {
		return nil, err
	}
	if res.Failed && res.Err != nil {
		return res.ReturnData, res.Err
	}
	return res.ReturnData, nil
}

// EstimateGas binary-searches gas for a message (latest state, no commit).
func (n *Node) EstimateGas(msg vm.Message) (uint64, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	lo, hi := uint64(21000), n.header.GasLimit
	if msg.GasLimit != 0 && msg.GasLimit < hi {
		hi = msg.GasLimit
	}
	cap := hi

	var lastErr error
	for lo+1 < hi {
		mid := (lo + hi) / 2
		snap := n.statedb.Snapshot()
		try := msg
		try.GasLimit = mid
		try.GasPrice = big.NewInt(0)
		try.NoFinalise = true
		if try.Value == nil {
			try.Value = uint256.NewInt(0)
		}
		exec := vm.NewExecutor(n.statedb, vm.BlockContext{
			Number:   n.header.Number,
			Time:     n.header.Timestamp,
			GasLimit: n.header.GasLimit,
			BaseFee:  n.baseFee,
			Coinbase: n.header.Proposer,
			ChainID:  n.chainID,
		})
		res, err := exec.ApplyMessage(try)
		n.statedb.RevertToSnapshot(snap)
		if err != nil || (res != nil && res.Failed) {
			lo = mid
			if res != nil {
				lastErr = res.Err
			} else {
				lastErr = err
			}
		} else {
			hi = mid
		}
	}
	// verify hi works
	snap := n.statedb.Snapshot()
	try := msg
	try.GasLimit = hi
	try.GasPrice = big.NewInt(0)
	try.NoFinalise = true
	if try.Value == nil {
		try.Value = uint256.NewInt(0)
	}
	exec := vm.NewExecutor(n.statedb, vm.BlockContext{
		Number:   n.header.Number,
		Time:     n.header.Timestamp,
		GasLimit: n.header.GasLimit,
		BaseFee:  n.baseFee,
		Coinbase: n.header.Proposer,
		ChainID:  n.chainID,
	})
	res, err := exec.ApplyMessage(try)
	n.statedb.RevertToSnapshot(snap)
	if err != nil || res.Failed {
		if lastErr != nil {
			return 0, lastErr
		}
		if err != nil {
			return 0, err
		}
		return 0, res.Err
	}
	if hi == cap && res.UsedGas < hi {
		// ok
	}
	// pad 20% for safety like geth estimate
	est := res.UsedGas * 12 / 10
	if est < 21000 {
		est = 21000
	}
	return est, nil
}

// FilterLogs returns logs matching basic address/topic filters in a block range.
func (n *Node) FilterLogs(fromBlock, toBlock uint64, addresses []dewcrypto.Address, topics [][]dewtypes.Hash) []*IndexedLog {
	n.mu.RLock()
	defer n.mu.RUnlock()
	var out []*IndexedLog
	addrSet := map[dewcrypto.Address]struct{}{}
	for _, a := range addresses {
		addrSet[a] = struct{}{}
	}
	for _, il := range n.allLogs {
		if il.BlockNumber < fromBlock || il.BlockNumber > toBlock {
			continue
		}
		if len(addrSet) > 0 {
			if _, ok := addrSet[il.Log.Address]; !ok {
				continue
			}
		}
		if !matchTopics(il.Log.Topics, topics) {
			continue
		}
		out = append(out, il)
	}
	return out
}

func matchTopics(logTopics []dewtypes.Hash, filter [][]dewtypes.Hash) bool {
	if len(filter) == 0 {
		return true
	}
	for i, alts := range filter {
		if len(alts) == 0 {
			continue // any
		}
		if i >= len(logTopics) {
			return false
		}
		ok := false
		for _, t := range alts {
			if t == logTopics[i] {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func effectiveGasPrice(tx *ethtypes.Transaction, baseFee *big.Int) *big.Int {
	switch tx.Type() {
	case ethtypes.DynamicFeeTxType:
		// effective = min(gasFeeCap, baseFee + gasTipCap)
		tip := tx.GasTipCap()
		feeCap := tx.GasFeeCap()
		if feeCap.Cmp(baseFee) < 0 {
			return nil
		}
		eff := new(big.Int).Add(baseFee, tip)
		if eff.Cmp(feeCap) > 0 {
			eff.Set(feeCap)
		}
		return eff
	default:
		return new(big.Int).Set(tx.GasPrice())
	}
}

func ethToDewAddr(a ethcommon.Address) dewcrypto.Address {
	var out dewcrypto.Address
	copy(out[:], a[:])
	return out
}