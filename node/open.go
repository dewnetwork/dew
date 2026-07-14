package node

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/core/state"
	dewtypes "github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/mempool"
	"github.com/dewnetwork/dew/params"
)

// DefaultDataDir is the default operator data directory (Linux FHS / container path).
// Layout: <datadir>/chaindata (Pebble), <datadir>/peers.json (P2P).
// Docker Compose volumes mount here so --datadir need not be set in command.
const DefaultDataDir = "/var/lib/dew"

// ChainDataDir returns <datadir>/chaindata for operator layouts.
func ChainDataDir(dataDir string) string {
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	return filepath.Join(dataDir, "chaindata")
}

// Open opens a durable Pebble-backed node from chainDataDir.
// Empty dir is initialized from genesis. Existing data must match genesis hash / chain ID.
func Open(g *config.Genesis, chainDataDir string) (*Node, error) {
	if g == nil {
		return nil, fmt.Errorf("node: nil genesis")
	}
	if chainDataDir == "" {
		return nil, fmt.Errorf("node: empty chain data dir")
	}
	if err := os.MkdirAll(chainDataDir, 0o755); err != nil {
		return nil, fmt.Errorf("node: mkdir chaindata: %w", err)
	}
	pdb, err := db.OpenPebble(chainDataDir)
	if err != nil {
		return nil, err
	}
	n, err := openWithDatabase(g, pdb)
	if err != nil {
		_ = pdb.Close()
		return nil, err
	}
	return n, nil
}

// NewFromGenesis initializes a fresh node under chainDataDir (must be empty of chain meta).
// Equivalent to Open on an empty directory.
func NewFromGenesis(g *config.Genesis, chainDataDir string) (*Node, error) {
	return Open(g, chainDataDir)
}

func openWithDatabase(g *config.Genesis, database db.Database) (*Node, error) {
	ver, hasMeta, err := readMetaVersion(database)
	if err != nil {
		return nil, err
	}
	if !hasMeta {
		return newNodeFromGenesisDB(g, database)
	}
	if ver != chainSchemaVersion {
		return nil, fmt.Errorf("node: unsupported chaindata schema version %d (want %d)", ver, chainSchemaVersion)
	}

	expectedHash, err := genesisBlockHash(g)
	if err != nil {
		return nil, err
	}
	storedHash, err := readMetaGenesisHash(database)
	if err != nil {
		return nil, fmt.Errorf("node: read genesis hash: %w", err)
	}
	if storedHash != expectedHash {
		return nil, fmt.Errorf("node: genesis hash mismatch: db=%s file=%s", storedHash.Hex(), expectedHash.Hex())
	}
	storedCID, err := readMetaChainID(database)
	if err != nil {
		return nil, fmt.Errorf("node: read chain id: %w", err)
	}
	if !chainIDEqual(storedCID, g.ChainID()) {
		return nil, fmt.Errorf("node: chain id mismatch: db=%s file=%s", storedCID, g.ChainID())
	}

	tipNum, tipHash, err := readTip(database)
	if err != nil {
		return nil, fmt.Errorf("node: read tip: %w", err)
	}

	statedb := state.New(database)
	n := newEmptyNode(g, database, statedb)
	if err := n.hydrateLocked(tipNum, tipHash); err != nil {
		return nil, err
	}
	return n, nil
}

func genesisBlockHash(g *config.Genesis) (dewtypes.Hash, error) {
	dir, err := os.MkdirTemp("", "dew-genesis-hash-*")
	if err != nil {
		return dewtypes.Hash{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	pdb, err := db.OpenPebble(filepath.Join(dir, "tmp"))
	if err != nil {
		return dewtypes.Hash{}, err
	}
	defer pdb.Close()
	block, _, err := g.Commit(pdb)
	if err != nil {
		return dewtypes.Hash{}, err
	}
	return block.Hash(), nil
}

func newNodeFromGenesisDB(g *config.Genesis, database db.Database) (*Node, error) {
	block, statedb, err := g.Commit(database)
	if err != nil {
		return nil, err
	}
	h := block.Header()
	n := newEmptyNode(g, database, statedb)
	n.header = h
	n.blocks[block.Hash()] = block
	n.blockNum[0] = block.Hash()
	if n.baseFee == nil {
		n.baseFee = big.NewInt(1_000_000_000)
	}
	if err := n.persistGenesisLocked(block); err != nil {
		return nil, fmt.Errorf("node: persist genesis: %w", err)
	}
	return n, nil
}

func newEmptyNode(g *config.Genesis, database db.Database, statedb *state.StateDB) *Node {
	h := &dewtypes.Header{BaseFee: big.NewInt(1_000_000_000)}
	n := &Node{
		genesis:           g,
		chainID:           g.ChainID(),
		db:                database,
		statedb:           statedb,
		header:            h,
		blocks:            make(map[dewtypes.Hash]*dewtypes.Block),
		blockNum:          make(map[uint64]dewtypes.Hash),
		txIndex:           make(map[dewtypes.Hash]*TxLookup),
		receipts:          make(map[dewtypes.Hash]*dewtypes.Receipt),
		gasPrice:          big.NewInt(1_000_000_000),
		baseFee:           big.NewInt(1_000_000_000),
		enableNative:      params.DefaultEnableNativePath,
		enablePrecompiles: params.DefaultEnableDewPrecompiles,
		enableStaking:     params.DefaultEnableStaking,
		pool:              mempool.New(mempool.DefaultConfig()),
		autoMine:          true,
	}
	if n.gasPrice != nil && n.gasPrice.Cmp(n.pool.Config().MinGasPriceWei) > 0 {
		cfg := mempool.DefaultConfig()
		cfg.MinGasPriceWei = new(big.Int).Set(n.gasPrice)
		n.pool = mempool.New(cfg)
	}
	return n
}

// hydrateLocked loads only the tip into memory (Track 4 lazy hydrate).
// Historical blocks, tx lookups, receipts, and logs load on demand from chaindata.
// No lock required during Open construction.
func (n *Node) hydrateLocked(tipNum uint64, tipHash dewtypes.Hash) error {
	tipBlock, err := n.readBlockFromDB(tipHash)
	if err != nil {
		return fmt.Errorf("node: hydrate tip %s: %w", tipHash.Hex(), err)
	}
	if tipBlock.Header().Number != tipNum {
		return fmt.Errorf("node: tip height mismatch meta=%d header=%d", tipNum, tipBlock.Header().Number)
	}
	n.blocks[tipHash] = tipBlock
	n.blockNum[tipNum] = tipHash
	// Tip only — do not preload 0..tip-1, tx index, receipts, or allLogs.
	n.header = tipBlock.Header().Copy()
	if n.header.BaseFee != nil {
		n.baseFee = new(big.Int).Set(n.header.BaseFee)
	}
	return nil
}

// readBlockFromDB loads header+body for hash without touching caches.
func (n *Node) readBlockFromDB(hash dewtypes.Hash) (*dewtypes.Block, error) {
	hdrBlob, err := n.db.Get(headerKey(hash))
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	hdr, err := decodeHeader(hdrBlob)
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	bodyBlob, err := n.db.Get(bodyKey(hash))
	if err != nil {
		return nil, fmt.Errorf("body: %w", err)
	}
	txs, err := decodeBody(bodyBlob)
	if err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}
	block := dewtypes.NewBlock(hdr, txs)
	if block.Hash() != hash {
		return nil, fmt.Errorf("block hash mismatch: got %s want %s", block.Hash().Hex(), hash.Hex())
	}
	return block, nil
}

// loadBlockByHashLocked returns a cached block or loads it from chaindata into the cache.
// Caller must hold n.mu (write).
func (n *Node) loadBlockByHashLocked(hash dewtypes.Hash) *dewtypes.Block {
	if b := n.blocks[hash]; b != nil {
		return b
	}
	if n.db == nil {
		return nil
	}
	block, err := n.readBlockFromDB(hash)
	if err != nil {
		return nil
	}
	n.blocks[hash] = block
	num := block.Header().Number
	if existing, ok := n.blockNum[num]; !ok || existing == hash {
		n.blockNum[num] = hash
	}
	return block
}

// loadBlockByNumberLocked returns a cached block or loads canonical height from chaindata.
// Caller must hold n.mu (write).
func (n *Node) loadBlockByNumberLocked(num uint64) *dewtypes.Block {
	if h, ok := n.blockNum[num]; ok {
		if b := n.blocks[h]; b != nil {
			return b
		}
	}
	if n.db == nil {
		return nil
	}
	hashBlob, err := n.db.Get(canonicalKey(num))
	if err != nil {
		return nil
	}
	hash := dewtypes.BytesToHash(hashBlob)
	n.blockNum[num] = hash
	return n.loadBlockByHashLocked(hash)
}

// loadTxLookupLocked returns a cached tx lookup or loads from chaindata.
// Caller must hold n.mu (write).
func (n *Node) loadTxLookupLocked(hash dewtypes.Hash) *TxLookup {
	if look := n.txIndex[hash]; look != nil {
		return look
	}
	if n.db == nil {
		return nil
	}
	blob, err := n.db.Get(txLookupKey(hash))
	if err != nil {
		return nil
	}
	look, err := decodeTxLookup(blob, hash)
	if err != nil {
		return nil
	}
	n.txIndex[hash] = look
	return look
}

// loadReceiptLocked returns a cached receipt or loads from chaindata.
// Caller must hold n.mu (write).
func (n *Node) loadReceiptLocked(hash dewtypes.Hash) *dewtypes.Receipt {
	if r := n.receipts[hash]; r != nil {
		return r
	}
	if n.db == nil {
		return nil
	}
	blob, err := n.db.Get(receiptKey(hash))
	if err != nil {
		return nil
	}
	r, err := decodeReceipt(blob)
	if err != nil {
		return nil
	}
	n.receipts[hash] = r
	return r
}

// cachedBlockCount reports how many blocks are currently in the RAM cache (tests / metrics).
func (n *Node) cachedBlockCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.blocks)
}

// Close releases the underlying database.
func (n *Node) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.db == nil {
		return nil
	}
	err := n.db.Close()
	n.db = nil
	return err
}
