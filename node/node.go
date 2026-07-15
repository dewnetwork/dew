// Package node is the in-process Dew full-node backend used by JSON-RPC (Phase A4+).
// Dev auto-mine packs ready pending txs (nonce chains + fee order) into sealed blocks.
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
	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/core/state"
	dewtypes "github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/core/vm"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/mempool"
)

// Node holds chain state and applies transactions sequentially.
type Node struct {
	mu sync.RWMutex

	genesis *config.Genesis
	chainID *big.Int
	db      db.Database
	statedb *state.StateDB
	header  *dewtypes.Header

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
	enableNativeSwap  bool // 0x101 orderbook methods (default off)
	peStats           vm.ExecutionStats

	// Phase C1: unified mempool admission (EVM + DewTx)
	pool *mempool.Pool

	// Dev auto-mine: seal a block of ready pending txs on admit (default true).
	autoMine bool

	// RPC subscription fan-out (WebSocket eth_subscribe). Separate from n.mu.
	eventMu   sync.Mutex
	eventSeq  uint64
	eventSubs map[uint64]*chainSub
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

// NewFromGenesis / Open are defined in open.go (Pebble-backed chaindata).

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

// MempoolTelemetry returns a read-only pool snapshot (size, fee floors, counters).
func (n *Node) MempoolTelemetry() mempool.Telemetry {
	n.mu.RLock()
	pool := n.pool
	n.mu.RUnlock()
	if pool == nil {
		return mempool.Telemetry{}
	}
	return pool.Telemetry()
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

// SetNativeSwapEnabled toggles live orderbook methods on 0x101.
func (n *Node) SetNativeSwapEnabled(v bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enableNativeSwap = v
}

// NativeSwapEnabled reports whether 0x101 orderbook methods are live.
func (n *Node) NativeSwapEnabled() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.enableNativeSwap
}

// stakingConfigFromGenesis maps genesis consensus fields onto the native module config.
func (n *Node) stakingConfigFromGenesis() native.StakingConfig {
	cfg := native.DefaultStakingConfig()
	if n.genesis == nil || n.genesis.Config == nil || n.genesis.Config.Consensus == nil {
		return cfg
	}
	c := n.genesis.Config.Consensus
	if c.EpochLength > 0 {
		cfg.EpochLength = c.EpochLength
	}
	if c.UnbondingPeriodSeconds > 0 {
		cfg.UnbondSeconds = c.UnbondingPeriodSeconds
	}
	if c.ActiveValidatorCap > 0 {
		cfg.ActiveCap = c.ActiveValidatorCap
	}
	if c.MinValidatorStake != "" {
		if v, ok := new(big.Int).SetString(c.MinValidatorStake, 0); ok && v.Sign() > 0 {
			cfg.MinSelfStake = v
		}
	}
	return cfg
}

// configureExecutor applies node feature flags and genesis staking params to a new VM executor.
func (n *Node) configureExecutor(exec *vm.Executor) {
	exec.EnableDewPrecompiles(n.enablePrecompiles)
	exec.EnableStaking(n.enableStaking)
	exec.EnableNativeSwap(n.enableNativeSwap)
	exec.SetStakingConfig(n.stakingConfigFromGenesis())
}

// TryRotateValidatorSet returns a new BFT set from staking ActiveSet at epoch
// boundaries when staking is enabled. nil,nil means keep the current set.
func (n *Node) TryRotateValidatorSet(height uint64) (*consensus.ValidatorSet, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if !n.enableStaking {
		return nil, nil
	}
	cfg := n.stakingConfigFromGenesis()
	if !consensus.ShouldRotateEpoch(height, cfg.EpochLength) {
		return nil, nil
	}
	mod := native.NewStakingModule(n.statedb, cfg)
	active := mod.ActiveSet()
	if len(active) == 0 {
		// No bonded candidates — keep genesis/static set.
		return nil, nil
	}
	return consensus.ActiveSetToValidatorSet(active)
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
// Historical heights load on demand from chaindata after lazy hydrate.
func (n *Node) GetBlockByNumber(num uint64) *dewtypes.Block {
	n.mu.RLock()
	if h, ok := n.blockNum[num]; ok {
		if b := n.blocks[h]; b != nil {
			n.mu.RUnlock()
			return b
		}
	}
	n.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()
	return n.loadBlockByNumberLocked(num)
}

// GetBlockByHash returns the block with the given hash.
// Misses load on demand from chaindata after lazy hydrate.
func (n *Node) GetBlockByHash(hash dewtypes.Hash) *dewtypes.Block {
	n.mu.RLock()
	if b := n.blocks[hash]; b != nil {
		n.mu.RUnlock()
		return b
	}
	n.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()
	return n.loadBlockByHashLocked(hash)
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

// GetTransaction returns indexed tx metadata (RAM cache or chaindata).
func (n *Node) GetTransaction(hash dewtypes.Hash) *TxLookup {
	n.mu.RLock()
	if look := n.txIndex[hash]; look != nil {
		n.mu.RUnlock()
		return look
	}
	n.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()
	return n.loadTxLookupLocked(hash)
}

// GetReceipt returns a receipt by tx hash (RAM cache or chaindata).
func (n *Node) GetReceipt(hash dewtypes.Hash) *dewtypes.Receipt {
	n.mu.RLock()
	if r := n.receipts[hash]; r != nil {
		n.mu.RUnlock()
		return r
	}
	n.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()
	return n.loadReceiptLocked(hash)
}

// SendDewRawTransaction decodes a signed DewTx, admits via mempool, and optionally auto-mines.
// Nonce rules match EVM: reject nonce < account; higher nonces stay pending (gap queue).
// With auto-mine, ready continuous nonce chains pack up to DefaultMaxTxsPerBlock (C1 residual).
// DewTx is still indexed via receipts/tx lookup; block body remains EVM-only under public-testnet-v1.
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

	accountNonce := n.statedb.GetNonce(tx.Sender)
	if tx.Nonce < accountNonce {
		return dewtypes.Hash{}, fmt.Errorf("nonce too low: got %d want >= %d", tx.Nonce, accountNonce)
	}

	// C1 admission (shared surface with EVM): size / fee / pool limits.
	hash, err := n.pool.AddDew(tx, raw, n.chainID)
	if err != nil {
		return dewtypes.Hash{}, err
	}
	if !n.autoMine {
		return hash, nil
	}
	// Future nonce only: stay pending until the gap is filled.
	if tx.Nonce > accountNonce {
		return hash, nil
	}
	if err := n.sealReadyDewFromPoolLocked(); err != nil {
		return hash, err
	}
	return hash, nil
}

// sealReadyDewFromPoolLocked packs ready pending DewTxs into one block and commits.
// Caller must hold n.mu. Body stays empty (no tagged-union wire yet); results go to tx index.
func (n *Node) sealReadyDewFromPoolLocked() error {
	parent := n.header.Copy()
	dtxs := n.selectPendingDewTxsForBlockLocked(DefaultMaxTxsPerBlock)
	if len(dtxs) == 0 {
		return nil
	}

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

	snap := n.statedb.Snapshot()
	results := make([]txExecResult, 0, len(dtxs))
	for i, dtx := range dtxs {
		res, err := n.executeDewTxLocked(newHeader, dtx, uint(i))
		if err != nil {
			n.statedb.RevertToSnapshot(snap)
			return err
		}
		results = append(results, res)
	}

	root, err := n.statedb.IntermediateRoot()
	if err != nil {
		n.statedb.RevertToSnapshot(snap)
		return err
	}
	newHeader.StateRoot = root
	newHeader.TxRoot = dewHashListRoot(results)

	block := dewtypes.NewBlock(newHeader, nil)
	if err := n.persistBlockLocked(block, results); err != nil {
		n.statedb.RevertToSnapshot(snap)
		return err
	}
	return nil
}

// SendRawTransaction decodes, admits via mempool, and optionally auto-mines.
// Nonce rules: reject only if nonce < account nonce (too low). Higher nonces are
// queued in the pool (gap queue). With auto-mine, a block is sealed when at least
// one ready (nonce-chain) tx exists — packing up to DefaultMaxTxsPerBlock.
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
	accountNonce := n.statedb.GetNonce(from)
	if tx.Nonce() < accountNonce {
		return dewtypes.Hash{}, fmt.Errorf("nonce too low: got %d want >= %d", tx.Nonce(), accountNonce)
	}

	// C1 admission (shared surface with DewTx): size / fee / pool limits / RBF.
	if _, err := n.pool.AddEVM(tx, from, raw, n.chainID); err != nil {
		return dewtypes.Hash{}, err
	}
	txHash := dewtypes.BytesToHash(tx.Hash().Bytes())
	if !n.autoMine {
		return txHash, nil
	}
	// Future nonce only: stay pending until the gap is filled.
	if tx.Nonce() > accountNonce {
		return txHash, nil
	}
	if err := n.sealReadyFromPoolLocked(); err != nil {
		// Tx remains admitted; surface seal failure to the caller.
		return dewtypes.Hash{}, err
	}
	return txHash, nil
}

// sealReadyFromPoolLocked packs ready pending EVM txs into one block and commits.
// Caller must hold n.mu. No-op (nil error) if nothing is executable yet.
func (n *Node) sealReadyFromPoolLocked() error {
	parent := n.header.Copy()
	txs := n.selectPendingTxsForBlockLocked(DefaultMaxTxsPerBlock, parent.GasLimit, nil)
	if len(txs) == 0 {
		return nil
	}

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

	snap := n.statedb.Snapshot()
	results, gasUsed, err := n.executeBlockTxsLocked(newHeader, txs)
	if err != nil {
		n.statedb.RevertToSnapshot(snap)
		return err
	}
	newHeader.GasUsed = gasUsed

	root, err := n.statedb.IntermediateRoot()
	if err != nil {
		n.statedb.RevertToSnapshot(snap)
		return err
	}
	newHeader.StateRoot = root
	newHeader.TxRoot = dewtypes.TxRoot(txs)

	block := dewtypes.NewBlock(newHeader, txs)
	if err := n.persistBlockLocked(block, results); err != nil {
		n.statedb.RevertToSnapshot(snap)
		return err
	}
	for _, res := range results {
		n.pool.Remove(res.txHash)
	}
	return nil
}

// TransactionsInBlock returns tx lookups for a block number.
// Uses the RAM cache plus an on-demand scan of durable tx lookups when needed.
func (n *Node) TransactionsInBlock(num uint64) []*TxLookup {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.lookupsForBlockLocked(num)
}

// lookupsForBlockLocked collects tx lookups for a height (cache + chaindata scan).
// Caller must hold n.mu (write).
func (n *Node) lookupsForBlockLocked(num uint64) []*TxLookup {
	seen := make(map[dewtypes.Hash]struct{})
	var out []*TxLookup
	add := func(look *TxLookup) {
		if look == nil {
			return
		}
		if _, ok := seen[look.TxHash]; ok {
			return
		}
		seen[look.TxHash] = struct{}{}
		out = append(out, look)
	}
	for _, look := range n.txIndex {
		if look != nil && look.BlockNumber == num {
			add(look)
		}
	}
	// EVM body txs (fast path when block is loadable).
	if blk := n.loadBlockByNumberLocked(num); blk != nil {
		for _, tx := range blk.Transactions() {
			if tx == nil {
				continue
			}
			add(n.loadTxLookupLocked(tx.Hash()))
		}
	}
	// DewTx (and any missed EVM) via durable index — needed after lazy open.
	if it, ok := n.db.(db.IteratePrefix); ok {
		_ = it.IteratePrefix([]byte{prefixTxLookup}, func(key, value []byte) error {
			if len(key) != 1+32 {
				return nil
			}
			txHash := dewtypes.BytesToHash(key[1:])
			if _, ok := seen[txHash]; ok {
				return nil
			}
			look, err := decodeTxLookup(value, txHash)
			if err != nil || look.BlockNumber != num {
				return nil
			}
			n.txIndex[txHash] = look
			add(look)
			return nil
		})
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
	n.configureExecutor(exec)
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
		n.configureExecutor(exec)
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
	n.configureExecutor(exec)
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
// Historical logs come from the secondary log index (O(range) by block prefix).
// Recent seals still feed allLogs for the in-process path; results are deduped.
func (n *Node) FilterLogs(fromBlock, toBlock uint64, addresses []dewcrypto.Address, topics [][]dewtypes.Hash) []*IndexedLog {
	n.mu.Lock()
	defer n.mu.Unlock()
	addrSet := map[dewcrypto.Address]struct{}{}
	for _, a := range addresses {
		addrSet[a] = struct{}{}
	}

	var out []*IndexedLog
	seen := make(map[string]struct{}) // txHash|logIndex
	add := func(il *IndexedLog) {
		if !logMatchesFilter(il, fromBlock, toBlock, addrSet, topics) {
			return
		}
		key := il.TxHash.Hex() + "|" + fmt.Sprint(il.Index)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, il)
	}

	for _, il := range n.allLogs {
		add(il)
	}

	// Durable secondary index (L|block|…) — not a full receipt prefix scan.
	if n.hasLogIndexVersion() {
		n.filterLogsFromIndexLocked(fromBlock, toBlock, add)
		return out
	}

	// Fallback for databases that never ran ensureLogIndex (should be rare).
	if it, ok := n.db.(db.IteratePrefix); ok {
		_ = it.IteratePrefix([]byte{prefixReceipt}, func(key, value []byte) error {
			if len(key) != 1+32 {
				return nil
			}
			txHash := dewtypes.BytesToHash(key[1:])
			rcpt, err := decodeReceipt(value)
			if err != nil {
				return nil
			}
			if rcpt.BlockNumber < fromBlock || rcpt.BlockNumber > toBlock {
				return nil
			}
			n.receipts[txHash] = rcpt
			for j, lg := range rcpt.Logs {
				add(&IndexedLog{
					Log:         lg,
					BlockNumber: rcpt.BlockNumber,
					BlockHash:   rcpt.BlockHash,
					TxHash:      txHash,
					TxIndex:     rcpt.TransactionIndex,
					Index:       uint(j),
				})
			}
			return nil
		})
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
