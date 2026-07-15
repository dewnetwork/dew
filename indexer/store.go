package indexer

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	_ "modernc.org/sqlite"
)

// Store is a SQLite-backed history index.
type Store struct {
	db *sql.DB
}

// OpenStore opens or creates a SQLite database at path.
func OpenStore(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("indexer: empty sqlite path")
	}
	// busy_timeout helps under concurrent read (HTTP) + write (ingest).
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS blocks (
  number INTEGER PRIMARY KEY,
  hash TEXT NOT NULL,
  timestamp INTEGER NOT NULL,
  tx_count INTEGER NOT NULL,
  gas_used INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS txs (
  hash TEXT PRIMARY KEY,
  block_number INTEGER NOT NULL,
  tx_index INTEGER NOT NULL,
  from_addr TEXT NOT NULL,
  to_addr TEXT,
  value TEXT NOT NULL,
  status INTEGER,
  gas_used INTEGER
);
CREATE INDEX IF NOT EXISTS idx_txs_from ON txs(from_addr, block_number DESC);
CREATE INDEX IF NOT EXISTS idx_txs_to ON txs(to_addr, block_number DESC);
CREATE INDEX IF NOT EXISTS idx_txs_block ON txs(block_number DESC);
CREATE TABLE IF NOT EXISTS transfers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  block_number INTEGER NOT NULL,
  tx_hash TEXT NOT NULL,
  log_index INTEGER NOT NULL,
  token TEXT NOT NULL,
  from_addr TEXT NOT NULL,
  to_addr TEXT NOT NULL,
  amount TEXT NOT NULL,
  UNIQUE(tx_hash, log_index)
);
CREATE INDEX IF NOT EXISTS idx_xfers_from ON transfers(from_addr, block_number DESC);
CREATE INDEX IF NOT EXISTS idx_xfers_to ON transfers(to_addr, block_number DESC);
CREATE INDEX IF NOT EXISTS idx_xfers_token ON transfers(token, block_number DESC);
CREATE TABLE IF NOT EXISTS contracts (
  address    TEXT PRIMARY KEY,
  name       TEXT,
  abi_json   TEXT NOT NULL,
  source     TEXT,
  compiler   TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
`)
	return err
}

// ContractRecord is a registered contract ABI (+ optional source).
type ContractRecord struct {
	Address   string
	Name      string
	ABIJSON   string
	Source    string
	Compiler  string
	CreatedAt int64
	UpdatedAt int64
}

// GetContract returns a registered contract by address (lowercase key).
func (s *Store) GetContract(addr string) (ContractRecord, bool, error) {
	a := normalizeAddr(addr)
	var r ContractRecord
	err := s.db.QueryRow(`
SELECT address, COALESCE(name,''), abi_json, COALESCE(source,''), COALESCE(compiler,''), created_at, updated_at
FROM contracts WHERE address = ?`, a).Scan(
		&r.Address, &r.Name, &r.ABIJSON, &r.Source, &r.Compiler, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return ContractRecord{}, false, nil
	}
	if err != nil {
		return ContractRecord{}, false, err
	}
	return r, true, nil
}

// UpsertContract inserts or replaces ABI/source for an address (preserves created_at).
func (s *Store) UpsertContract(r ContractRecord) error {
	a := normalizeAddr(r.Address)
	if a == "" {
		return fmt.Errorf("empty address")
	}
	if utf8.RuneCountInString(r.Name) > 128 {
		return fmt.Errorf("name too long")
	}
	if utf8.RuneCountInString(r.Compiler) > 64 {
		return fmt.Errorf("compiler too long")
	}
	now := time.Now().Unix()
	_, err := s.db.Exec(`
INSERT INTO contracts(address, name, abi_json, source, compiler, created_at, updated_at)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(address) DO UPDATE SET
  name=excluded.name,
  abi_json=excluded.abi_json,
  source=excluded.source,
  compiler=excluded.compiler,
  updated_at=excluded.updated_at`,
		a, r.Name, r.ABIJSON, r.Source, r.Compiler, now, now,
	)
	return err
}

func (s *Store) getMeta(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) setMeta(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO meta(key,value) VALUES(?,?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// LastIndexedBlock returns the highest fully indexed height (0 if none beyond genesis tracking).
func (s *Store) LastIndexedBlock() (uint64, error) {
	v, ok, err := s.getMeta("last_indexed_block")
	if err != nil {
		return 0, err
	}
	if !ok || v == "" {
		return 0, nil
	}
	var n uint64
	_, err = fmt.Sscanf(v, "%d", &n)
	return n, err
}

func (s *Store) setLastIndexedBlock(n uint64) error {
	return s.setMeta("last_indexed_block", fmt.Sprintf("%d", n))
}

// BlockRow is a stored block summary.
type BlockRow struct {
	Number    uint64
	Hash      string
	Timestamp uint64
	TxCount   int
	GasUsed   uint64
}

// TxRow is a stored transaction.
type TxRow struct {
	Hash        string `json:"hash"`
	BlockNumber uint64 `json:"blockNumber"`
	TxIndex     int    `json:"transactionIndex"`
	From        string `json:"from"`
	To          string `json:"to,omitempty"`
	Value       string `json:"value"`
	Status      *int   `json:"status,omitempty"`
	GasUsed     *uint64 `json:"gasUsed,omitempty"`
}

// TransferRow is an ERC-20 Transfer log.
type TransferRow struct {
	BlockNumber uint64 `json:"blockNumber"`
	TxHash      string `json:"txHash"`
	LogIndex    int    `json:"logIndex"`
	Token       string `json:"token"`
	From        string `json:"from"`
	To          string `json:"to"`
	Amount      string `json:"amount"`
}

// VolumePoint is tx volume for a time or block bucket.
type VolumePoint struct {
	DayKey    string `json:"dayKey,omitempty"`
	BlockFrom uint64 `json:"blockFrom,omitempty"`
	BlockTo   uint64 `json:"blockTo,omitempty"`
	TxCount   int    `json:"txCount"`
	GasUsed   uint64 `json:"gasUsed"`
	Timestamp uint64 `json:"timestamp,omitempty"`
}

func normalizeAddr(a string) string {
	return strings.ToLower(strings.TrimSpace(a))
}

func (s *Store) insertBlock(tx *sql.Tx, b BlockRow) error {
	_, err := tx.Exec(
		`INSERT INTO blocks(number,hash,timestamp,tx_count,gas_used) VALUES(?,?,?,?,?)
ON CONFLICT(number) DO UPDATE SET hash=excluded.hash, timestamp=excluded.timestamp,
tx_count=excluded.tx_count, gas_used=excluded.gas_used`,
		b.Number, b.Hash, b.Timestamp, b.TxCount, b.GasUsed,
	)
	return err
}

func (s *Store) insertTx(tx *sql.Tx, r TxRow) error {
	var to any
	if r.To != "" {
		to = r.To
	}
	var status any
	if r.Status != nil {
		status = *r.Status
	}
	var gas any
	if r.GasUsed != nil {
		gas = *r.GasUsed
	}
	_, err := tx.Exec(
		`INSERT INTO txs(hash,block_number,tx_index,from_addr,to_addr,value,status,gas_used)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(hash) DO UPDATE SET block_number=excluded.block_number, tx_index=excluded.tx_index,
from_addr=excluded.from_addr, to_addr=excluded.to_addr, value=excluded.value,
status=excluded.status, gas_used=excluded.gas_used`,
		r.Hash, r.BlockNumber, r.TxIndex, r.From, to, r.Value, status, gas,
	)
	return err
}

func (s *Store) insertTransfer(tx *sql.Tx, r TransferRow) error {
	_, err := tx.Exec(
		`INSERT INTO transfers(block_number,tx_hash,log_index,token,from_addr,to_addr,amount)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(tx_hash,log_index) DO UPDATE SET block_number=excluded.block_number,
token=excluded.token, from_addr=excluded.from_addr, to_addr=excluded.to_addr, amount=excluded.amount`,
		r.BlockNumber, r.TxHash, r.LogIndex, r.Token, r.From, r.To, r.Amount,
	)
	return err
}

// AddressTxs returns txs where addr is from or to, newest first.
func (s *Store) AddressTxs(addr string, limit, offset int) ([]TxRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	a := normalizeAddr(addr)
	rows, err := s.db.Query(`
SELECT hash, block_number, tx_index, from_addr, COALESCE(to_addr,''), value, status, gas_used
FROM txs
WHERE from_addr = ? OR to_addr = ?
ORDER BY block_number DESC, tx_index DESC
LIMIT ? OFFSET ?`, a, a, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTxs(rows)
}

func scanTxs(rows *sql.Rows) ([]TxRow, error) {
	var out []TxRow
	for rows.Next() {
		var r TxRow
		var to string
		var status sql.NullInt64
		var gas sql.NullInt64
		if err := rows.Scan(&r.Hash, &r.BlockNumber, &r.TxIndex, &r.From, &to, &r.Value, &status, &gas); err != nil {
			return nil, err
		}
		r.To = to
		if status.Valid {
			v := int(status.Int64)
			r.Status = &v
		}
		if gas.Valid {
			v := uint64(gas.Int64)
			r.GasUsed = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AddressTransfers returns ERC-20 transfers involving addr.
func (s *Store) AddressTransfers(addr string, limit, offset int) ([]TransferRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	a := normalizeAddr(addr)
	rows, err := s.db.Query(`
SELECT block_number, tx_hash, log_index, token, from_addr, to_addr, amount
FROM transfers
WHERE from_addr = ? OR to_addr = ?
ORDER BY block_number DESC, log_index DESC
LIMIT ? OFFSET ?`, a, a, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TransferRow
	for rows.Next() {
		var r TransferRow
		if err := rows.Scan(&r.BlockNumber, &r.TxHash, &r.LogIndex, &r.Token, &r.From, &r.To, &r.Amount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// VolumeByDay aggregates tx_count by UTC day between fromBlock and toBlock inclusive.
func (s *Store) VolumeByDay(fromBlock, toBlock uint64) ([]VolumePoint, error) {
	if toBlock < fromBlock {
		return nil, nil
	}
	rows, err := s.db.Query(`
SELECT date(timestamp, 'unixepoch') AS day,
       SUM(tx_count), SUM(gas_used), MIN(timestamp), MIN(number), MAX(number)
FROM blocks
WHERE number >= ? AND number <= ?
GROUP BY day
ORDER BY day ASC`, fromBlock, toBlock)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VolumePoint
	for rows.Next() {
		var p VolumePoint
		var day string
		var txSum, gasSum int64
		var ts, bf, bt int64
		if err := rows.Scan(&day, &txSum, &gasSum, &ts, &bf, &bt); err != nil {
			return nil, err
		}
		p.DayKey = day
		p.TxCount = int(txSum)
		p.GasUsed = uint64(gasSum)
		p.Timestamp = uint64(ts)
		p.BlockFrom = uint64(bf)
		p.BlockTo = uint64(bt)
		out = append(out, p)
	}
	return out, rows.Err()
}

// MaxBlock returns MAX(number) from blocks table.
func (s *Store) MaxBlock() (uint64, error) {
	var n sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(number) FROM blocks`).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 0, nil
	}
	return uint64(n.Int64), nil
}
