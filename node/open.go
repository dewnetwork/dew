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

// hydrateLocked loads canonical chain 0..tip into memory maps. No lock required during Open construction.
func (n *Node) hydrateLocked(tipNum uint64, tipHash dewtypes.Hash) error {
	for i := uint64(0); i <= tipNum; i++ {
		hashBlob, err := n.db.Get(canonicalKey(i))
		if err != nil {
			return fmt.Errorf("node: canonical %d: %w", i, err)
		}
		hash := dewtypes.BytesToHash(hashBlob)
		hdrBlob, err := n.db.Get(headerKey(hash))
		if err != nil {
			return fmt.Errorf("node: header %s: %w", hash.Hex(), err)
		}
		hdr, err := decodeHeader(hdrBlob)
		if err != nil {
			return fmt.Errorf("node: decode header %s: %w", hash.Hex(), err)
		}
		bodyBlob, err := n.db.Get(bodyKey(hash))
		if err != nil {
			return fmt.Errorf("node: body %s: %w", hash.Hex(), err)
		}
		txs, err := decodeBody(bodyBlob)
		if err != nil {
			return fmt.Errorf("node: decode body %s: %w", hash.Hex(), err)
		}
		block := dewtypes.NewBlock(hdr, txs)
		if block.Hash() != hash {
			return fmt.Errorf("node: block hash mismatch at %d: got %s want %s", i, block.Hash().Hex(), hash.Hex())
		}
		n.blocks[hash] = block
		n.blockNum[i] = hash
	}

	if it, ok := n.db.(db.IteratePrefix); ok {
		err := it.IteratePrefix([]byte{prefixTxLookup}, func(key, value []byte) error {
			if len(key) != 1+32 {
				return nil
			}
			txHash := dewtypes.BytesToHash(key[1:])
			look, err := decodeTxLookup(value, txHash)
			if err != nil {
				return err
			}
			n.txIndex[txHash] = look
			return nil
		})
		if err != nil {
			return fmt.Errorf("node: hydrate tx index: %w", err)
		}
		err = it.IteratePrefix([]byte{prefixReceipt}, func(key, value []byte) error {
			if len(key) != 1+32 {
				return nil
			}
			txHash := dewtypes.BytesToHash(key[1:])
			rcpt, err := decodeReceipt(value)
			if err != nil {
				return err
			}
			n.receipts[txHash] = rcpt
			for j, lg := range rcpt.Logs {
				n.allLogs = append(n.allLogs, &IndexedLog{
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
		if err != nil {
			return fmt.Errorf("node: hydrate receipts: %w", err)
		}
	}

	tipBlock := n.blocks[tipHash]
	if tipBlock == nil {
		return fmt.Errorf("node: tip block %s missing after hydrate", tipHash.Hex())
	}
	n.header = tipBlock.Header().Copy()
	if n.header.BaseFee != nil {
		n.baseFee = new(big.Int).Set(n.header.BaseFee)
	}
	if tipNum != n.header.Number {
		return fmt.Errorf("node: tip height mismatch meta=%d header=%d", tipNum, n.header.Number)
	}
	return nil
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
