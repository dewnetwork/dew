package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dewnetwork/dew/crypto"
)

// ERC-20 Transfer(address,address,uint256) topic0.
var transferTopic0 = "0x" + fmt.Sprintf("%x", crypto.Keccak256([]byte("Transfer(address,address,uint256)")))

// Config for the indexer service.
type Config struct {
	RPCURL     string
	SQLitePath string
	ListenAddr string
	// StartBlock is the first block to index if the DB is empty (default 0).
	StartBlock uint64
	// PollInterval between tip checks after catch-up.
	PollInterval time.Duration
	// MaxBlocksPerTick limits how many blocks to index per loop (backpressure).
	MaxBlocksPerTick int
}

// DefaultConfig returns Path B–friendly defaults.
func DefaultConfig() Config {
	return Config{
		RPCURL:           "http://127.0.0.1:8545",
		SQLitePath:       "indexer.db",
		ListenAddr:       "127.0.0.1:8550",
		StartBlock:       0,
		PollInterval:     time.Second,
		MaxBlocksPerTick: 64,
	}
}

// Indexer runs ingest + optional HTTP API.
type Indexer struct {
	cfg    Config
	store  *Store
	rpc    *rpcClient
	tip    atomic.Uint64
	indexed atomic.Uint64
}

// New creates an Indexer with open store.
func New(cfg Config) (*Indexer, error) {
	if cfg.RPCURL == "" {
		cfg.RPCURL = DefaultConfig().RPCURL
	}
	if cfg.SQLitePath == "" {
		cfg.SQLitePath = DefaultConfig().SQLitePath
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if cfg.MaxBlocksPerTick <= 0 {
		cfg.MaxBlocksPerTick = 64
	}
	st, err := OpenStore(cfg.SQLitePath)
	if err != nil {
		return nil, err
	}
	ix := &Indexer{
		cfg:   cfg,
		store: st,
		rpc:   newRPCClient(cfg.RPCURL),
	}
	last, _ := st.LastIndexedBlock()
	ix.indexed.Store(last)
	return ix, nil
}

// Close releases resources.
func (ix *Indexer) Close() error {
	return ix.store.Close()
}

// Status is a snapshot for /v1/status.
type Status struct {
	RPCURL          string `json:"rpcUrl"`
	ChainID         string `json:"chainId,omitempty"`
	Tip             uint64 `json:"tip"`
	Indexed         uint64 `json:"indexed"`
	Lag             uint64 `json:"lag"`
	TransferTopic0  string `json:"transferTopic0"`
}

// Status returns current tip/indexed lag.
func (ix *Indexer) Status(ctx context.Context) (Status, error) {
	tip, err := ix.rpc.blockNumber()
	if err != nil {
		tip = ix.tip.Load()
	} else {
		ix.tip.Store(tip)
	}
	indexed := ix.indexed.Load()
	cid, _ := ix.rpc.chainID()
	var lag uint64
	if tip > indexed {
		lag = tip - indexed
	}
	return Status{
		RPCURL:         ix.cfg.RPCURL,
		ChainID:        cid,
		Tip:            tip,
		Indexed:        indexed,
		Lag:            lag,
		TransferTopic0: transferTopic0,
	}, nil
}

// RunIngest blocks until ctx is cancelled, continuously indexing toward tip.
func (ix *Indexer) RunIngest(ctx context.Context) error {
	// Ensure we start from StartBlock if empty.
	last, err := ix.store.LastIndexedBlock()
	if err != nil {
		return err
	}
	if last == 0 {
		// Check if any blocks stored — if empty, start at StartBlock-1 so next is StartBlock.
		max, err := ix.store.MaxBlock()
		if err != nil {
			return err
		}
		if max == 0 {
			if ix.cfg.StartBlock > 0 {
				// Pretend we finished StartBlock-1
				last = ix.cfg.StartBlock - 1
				// if StartBlock is 0, last stays 0 and we index 0 on first run after checking
			}
		}
	}

	ticker := time.NewTicker(ix.cfg.PollInterval)
	defer ticker.Stop()

	for {
		if err := ix.catchUp(ctx); err != nil && ctx.Err() == nil {
			log.Printf("indexer: catch-up: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (ix *Indexer) catchUp(ctx context.Context) error {
	tip, err := ix.rpc.blockNumber()
	if err != nil {
		return err
	}
	ix.tip.Store(tip)

	last, err := ix.store.LastIndexedBlock()
	if err != nil {
		return err
	}
	_, hasMeta, err := ix.store.getMeta("last_indexed_block")
	if err != nil {
		return err
	}
	var next uint64
	if !hasMeta {
		// Brand-new DB: index from StartBlock (include genesis when 0).
		next = ix.cfg.StartBlock
	} else {
		next = last + 1
	}

	n := 0
	for next <= tip && n < ix.cfg.MaxBlocksPerTick {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := ix.indexBlock(next); err != nil {
			return fmt.Errorf("block %d: %w", next, err)
		}
		ix.indexed.Store(next)
		next++
		n++
	}
	return nil
}

// CatchUpOnce indexes up to tip (or MaxBlocksPerTick * many) for tests — runs until caught up or maxRounds.
func (ix *Indexer) CatchUpOnce(ctx context.Context) error {
	for i := 0; i < 10_000; i++ {
		before := ix.indexed.Load()
		if err := ix.catchUp(ctx); err != nil {
			return err
		}
		tip := ix.tip.Load()
		if ix.indexed.Load() >= tip {
			return nil
		}
		if ix.indexed.Load() == before {
			// no progress
			return nil
		}
	}
	return fmt.Errorf("indexer: catch-up exceeded rounds")
}

func (ix *Indexer) indexBlock(num uint64) error {
	blk, err := ix.rpc.getBlock(num, true)
	if err != nil {
		return err
	}
	if blk == nil {
		return fmt.Errorf("block %d not found", num)
	}
	ts, err := parseHexUint64(blk.Timestamp)
	if err != nil {
		return err
	}
	gasUsed, err := parseHexUint64(blk.GasUsed)
	if err != nil {
		return err
	}

	// Parse transactions (full objects or hashes).
	var txHashes []string
	var fullTxs []rpcTx
	if len(blk.Transactions) > 0 && string(blk.Transactions) != "null" {
		if err := json.Unmarshal(blk.Transactions, &fullTxs); err != nil {
			// try hash-only array
			if err2 := json.Unmarshal(blk.Transactions, &txHashes); err2 != nil {
				return fmt.Errorf("transactions: %v / %v", err, err2)
			}
		} else {
			for _, t := range fullTxs {
				txHashes = append(txHashes, t.Hash)
			}
		}
	}

	txMap := map[string]rpcTx{}
	for _, t := range fullTxs {
		txMap[strings.ToLower(t.Hash)] = t
	}

	sqlTx, err := ix.store.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = sqlTx.Rollback() }()

	br := BlockRow{
		Number:    num,
		Hash:      strings.ToLower(blk.Hash),
		Timestamp: ts,
		TxCount:   len(txHashes),
		GasUsed:   gasUsed,
	}
	if err := ix.store.insertBlock(sqlTx, br); err != nil {
		return err
	}

	for i, h := range txHashes {
		h = strings.ToLower(h)
		row := TxRow{
			Hash:        h,
			BlockNumber: num,
			TxIndex:     i,
			Value:       "0x0",
		}
		if ft, ok := txMap[h]; ok {
			row.From = normalizeAddr(ft.From)
			if ft.To != nil {
				row.To = normalizeAddr(*ft.To)
			}
			if ft.Value != "" {
				row.Value = ft.Value
			}
			if ft.Index != "" {
				if idx, err := parseHexUint64(ft.Index); err == nil {
					row.TxIndex = int(idx)
				}
			}
		}
		rcpt, err := ix.rpc.getReceipt(h)
		if err != nil {
			return err
		}
		if rcpt != nil {
			if rcpt.From != "" {
				row.From = normalizeAddr(rcpt.From)
			}
			if rcpt.To != nil {
				row.To = normalizeAddr(*rcpt.To)
			}
			if st, err := parseHexUint64(rcpt.Status); err == nil {
				v := int(st)
				row.Status = &v
			}
			if gu, err := parseHexUint64(rcpt.GasUsed); err == nil {
				row.GasUsed = &gu
			}
			if idx, err := parseHexUint64(rcpt.TransactionIndex); err == nil {
				row.TxIndex = int(idx)
			}
			for _, lg := range rcpt.Logs {
				if err := ix.indexLog(sqlTx, num, h, lg); err != nil {
					return err
				}
			}
		}
		if row.From == "" {
			row.From = "0x0000000000000000000000000000000000000000"
		}
		if err := ix.store.insertTx(sqlTx, row); err != nil {
			return err
		}
	}

	if err := ix.store.setMetaTx(sqlTx, "last_indexed_block", fmt.Sprintf("%d", num)); err != nil {
		return err
	}
	return sqlTx.Commit()
}

func (s *Store) setMetaTx(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec(`INSERT INTO meta(key,value) VALUES(?,?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (ix *Indexer) indexLog(tx *sql.Tx, blockNum uint64, txHash string, lg rpcLog) error {
	if len(lg.Topics) == 0 {
		return nil
	}
	t0 := strings.ToLower(lg.Topics[0])
	if t0 != transferTopic0 {
		return nil
	}
	if len(lg.Topics) < 3 {
		return nil
	}
	logIdx, err := parseHexUint64(lg.LogIndex)
	if err != nil {
		return err
	}
	from := normalizeAddr(topicWordToAddr(lg.Topics[1]))
	to := normalizeAddr(topicWordToAddr(lg.Topics[2]))
	amount := lg.Data
	if amount == "" {
		amount = "0x0"
	}
	return ix.store.insertTransfer(tx, TransferRow{
		BlockNumber: blockNum,
		TxHash:      strings.ToLower(txHash),
		LogIndex:    int(logIdx),
		Token:       normalizeAddr(lg.Address),
		From:        from,
		To:          to,
		Amount:      amount,
	})
}
