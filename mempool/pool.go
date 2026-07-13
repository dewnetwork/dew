package mempool

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	dewtypes "github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// Kind distinguishes EVM vs DewTx entries.
type Kind uint8

const (
	KindEVM Kind = iota
	KindDew
)

// Errors returned by admission.
var (
	ErrPoolFull       = errors.New("mempool: pool full")
	ErrSenderLimit    = errors.New("mempool: per-sender limit")
	ErrTxTooLarge     = errors.New("mempool: transaction too large")
	ErrUnderpriced    = errors.New("mempool: underpriced")
	ErrReplaceUnder   = errors.New("mempool: replacement underpriced")
	ErrAlreadyKnown   = errors.New("mempool: already known")
	ErrRBFDisabled    = errors.New("mempool: replace-by-fee disabled")
	ErrInvalidTx      = errors.New("mempool: invalid transaction")
	ErrWrongChain     = errors.New("mempool: wrong chain id")
	ErrNilConfig      = errors.New("mempool: nil config fields")
)

// Entry is one admitted pending transaction.
type Entry struct {
	Hash      dewtypes.Hash
	From      crypto.Address
	Nonce     uint64
	Kind      Kind
	Raw       []byte
	// Price is the comparable fee metric: EVM gas price / fee cap (wei), or Dew flat fee.
	Price *big.Int
	// Tip is EIP-1559 tip (nil for legacy / Dew).
	Tip *big.Int
	Size    int
	AddedAt time.Time
}

// RejectCounters tracks admission rejections by reason (read-only telemetry).
type RejectCounters struct {
	PoolFull      uint64 `json:"pool_full"`
	SenderLimit   uint64 `json:"sender_limit"`
	Underpriced   uint64 `json:"underpriced"`
	ReplaceUnder  uint64 `json:"replace_underpriced"`
	AlreadyKnown  uint64 `json:"already_known"`
	TxTooLarge    uint64 `json:"tx_too_large"`
	Invalid       uint64 `json:"invalid"`
	WrongChain    uint64 `json:"wrong_chain"`
	RBFDisabled   uint64 `json:"rbf_disabled"`
	Total         uint64 `json:"total"`
}

// SenderPending is one address's pending count for telemetry top-N lists.
type SenderPending struct {
	Address crypto.Address `json:"address"`
	Pending int            `json:"pending"`
}

// Telemetry is a read-only snapshot of pool size, fee floors, and counters.
// Admission policy is not changed by reading this.
type Telemetry struct {
	Pending          int            `json:"pending"`
	Senders          int            `json:"senders"`
	PendingEVM       int            `json:"pending_evm"`
	PendingDew       int            `json:"pending_dew"`
	MaxGlobal        int            `json:"max_global"`
	MaxPerSender     int            `json:"max_per_sender"`
	MaxTxBytes       int            `json:"max_tx_bytes"`
	MinGasPriceWei   string         `json:"min_gas_price_wei"`
	MinTipWei        string         `json:"min_tip_wei"`
	MinDewFeeWei     uint64         `json:"min_dew_fee_wei"`
	PriceBumpPercent uint64         `json:"price_bump_percent"`
	Admits           uint64         `json:"admits"`
	Replaces         uint64         `json:"replaces"`
	Evictions        uint64         `json:"evictions"`
	Rejects          RejectCounters `json:"rejects"`
	// TopSenders is at most 16 senders with the most pending txs (desc).
	TopSenders []SenderPending `json:"top_senders"`
}

// Pool is a unified mempool for EVM and DewTx.
type Pool struct {
	cfg Config

	mu      sync.RWMutex
	byHash  map[dewtypes.Hash]*Entry
	// bySender maps sender → nonce → hash
	bySender map[crypto.Address]map[uint64]dewtypes.Hash

	// Lifetime counters (not reset on Remove). Protected by mu.
	admits    uint64
	replaces  uint64
	evictions uint64
	rejects   RejectCounters
}

// New creates a pool with cfg (defaults applied for zero fields).
func New(cfg Config) *Pool {
	d := DefaultConfig()
	if cfg.MaxGlobal <= 0 {
		cfg.MaxGlobal = d.MaxGlobal
	}
	if cfg.MaxPerSender <= 0 {
		cfg.MaxPerSender = d.MaxPerSender
	}
	if cfg.MaxTxBytes <= 0 {
		cfg.MaxTxBytes = d.MaxTxBytes
	}
	if cfg.MinGasPriceWei == nil {
		cfg.MinGasPriceWei = new(big.Int).Set(d.MinGasPriceWei)
	}
	if cfg.MinTipWei == nil {
		cfg.MinTipWei = new(big.Int).Set(d.MinTipWei)
	}
	if cfg.MinDewFeeWei == 0 {
		cfg.MinDewFeeWei = d.MinDewFeeWei
	}
	// PriceBumpPercent 0 is valid (RBF off); only default when negative would be wrong — keep as-is.
	return &Pool{
		cfg:      cfg,
		byHash:   make(map[dewtypes.Hash]*Entry),
		bySender: make(map[crypto.Address]map[uint64]dewtypes.Hash),
	}
}

// Config returns a copy of the active config.
func (p *Pool) Config() Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c := p.cfg
	if c.MinGasPriceWei != nil {
		c.MinGasPriceWei = new(big.Int).Set(c.MinGasPriceWei)
	}
	if c.MinTipWei != nil {
		c.MinTipWei = new(big.Int).Set(c.MinTipWei)
	}
	return c
}

// Len returns the number of pending txs.
func (p *Pool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.byHash)
}

// Get returns an entry by hash.
func (p *Pool) Get(hash dewtypes.Hash) *Entry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.byHash[hash]
}

// Remove deletes a hash if present.
func (p *Pool) Remove(hash dewtypes.Hash) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.removeLocked(hash)
}

func (p *Pool) removeLocked(hash dewtypes.Hash) {
	e, ok := p.byHash[hash]
	if !ok {
		return
	}
	delete(p.byHash, hash)
	if m, ok := p.bySender[e.From]; ok {
		delete(m, e.Nonce)
		if len(m) == 0 {
			delete(p.bySender, e.From)
		}
	}
}

// AddEVM admits a decoded EVM transaction.
// chainID is the local chain id; baseFee is used only for documentation of effective price checks
// (admission uses fee cap / gas price floors, not baseFee market simulation).
func (p *Pool) AddEVM(tx *ethtypes.Transaction, from crypto.Address, raw []byte, chainID *big.Int) (dewtypes.Hash, error) {
	if tx == nil {
		p.noteReject(ErrInvalidTx)
		return dewtypes.Hash{}, ErrInvalidTx
	}
	if len(raw) == 0 {
		var err error
		raw, err = tx.MarshalBinary()
		if err != nil {
			wrapped := fmt.Errorf("%w: marshal: %v", ErrInvalidTx, err)
			p.noteReject(ErrInvalidTx)
			return dewtypes.Hash{}, wrapped
		}
	}
	if len(raw) > p.cfg.MaxTxBytes {
		p.noteReject(ErrTxTooLarge)
		return dewtypes.Hash{}, ErrTxTooLarge
	}
	if tx.ChainId() != nil && tx.ChainId().Sign() != 0 && chainID != nil && tx.ChainId().Cmp(chainID) != 0 {
		p.noteReject(ErrWrongChain)
		return dewtypes.Hash{}, ErrWrongChain
	}

	price, tip, err := p.evmPrice(tx)
	if err != nil {
		p.noteReject(err)
		return dewtypes.Hash{}, err
	}

	hash := dewtypes.BytesToHash(tx.Hash().Bytes())
	e := &Entry{
		Hash:    hash,
		From:    from,
		Nonce:   tx.Nonce(),
		Kind:    KindEVM,
		Raw:     append([]byte(nil), raw...),
		Price:   price,
		Tip:     tip,
		Size:    len(raw),
		AddedAt: time.Now(),
	}
	return p.add(e)
}

// AddDew admits a decoded DewTx.
func (p *Pool) AddDew(tx *dewtypes.DewTx, raw []byte, chainID *big.Int) (dewtypes.Hash, error) {
	if tx == nil {
		p.noteReject(ErrInvalidTx)
		return dewtypes.Hash{}, ErrInvalidTx
	}
	if len(raw) == 0 {
		var err error
		raw, err = tx.MarshalBinary()
		if err != nil {
			wrapped := fmt.Errorf("%w: marshal: %v", ErrInvalidTx, err)
			p.noteReject(ErrInvalidTx)
			return dewtypes.Hash{}, wrapped
		}
	}
	if len(raw) > p.cfg.MaxTxBytes {
		p.noteReject(ErrTxTooLarge)
		return dewtypes.Hash{}, ErrTxTooLarge
	}
	if tx.ChainID == nil || (chainID != nil && tx.ChainID.Cmp(chainID) != 0) {
		p.noteReject(ErrWrongChain)
		return dewtypes.Hash{}, ErrWrongChain
	}

	fee := tx.Fee
	if fee == 0 {
		// Protocol substitutes default; admission still requires min floor.
		fee = p.cfg.MinDewFeeWei
	}
	if fee < p.cfg.MinDewFeeWei {
		p.noteReject(ErrUnderpriced)
		return dewtypes.Hash{}, ErrUnderpriced
	}

	hash := tx.Hash()
	e := &Entry{
		Hash:    hash,
		From:    tx.Sender,
		Nonce:   tx.Nonce,
		Kind:    KindDew,
		Raw:     append([]byte(nil), raw...),
		Price:   new(big.Int).SetUint64(fee),
		Size:    len(raw),
		AddedAt: time.Now(),
	}
	return p.add(e)
}

func (p *Pool) evmPrice(tx *ethtypes.Transaction) (price, tip *big.Int, err error) {
	switch tx.Type() {
	case ethtypes.DynamicFeeTxType:
		feeCap := tx.GasFeeCap()
		tipCap := tx.GasTipCap()
		if feeCap == nil || tipCap == nil {
			return nil, nil, ErrInvalidTx
		}
		if feeCap.Cmp(p.cfg.MinGasPriceWei) < 0 {
			return nil, nil, ErrUnderpriced
		}
		if tipCap.Cmp(p.cfg.MinTipWei) < 0 {
			return nil, nil, ErrUnderpriced
		}
		return new(big.Int).Set(feeCap), new(big.Int).Set(tipCap), nil
	default:
		gp := tx.GasPrice()
		if gp == nil || gp.Cmp(p.cfg.MinGasPriceWei) < 0 {
			return nil, nil, ErrUnderpriced
		}
		return new(big.Int).Set(gp), nil, nil
	}
}

func (p *Pool) add(e *Entry) (dewtypes.Hash, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.byHash[e.Hash]; ok {
		p.noteRejectLocked(ErrAlreadyKnown)
		return e.Hash, ErrAlreadyKnown
	}

	replaced := false
	// Same sender+nonce → replace-by-fee or reject.
	if m, ok := p.bySender[e.From]; ok {
		if oldHash, exists := m[e.Nonce]; exists {
			old := p.byHash[oldHash]
			if old == nil {
				delete(m, e.Nonce)
			} else {
				if p.cfg.PriceBumpPercent == 0 {
					p.noteRejectLocked(ErrRBFDisabled)
					return dewtypes.Hash{}, ErrRBFDisabled
				}
				// Require price * (100+bump)/100
				min := new(big.Int).Mul(old.Price, big.NewInt(int64(100+p.cfg.PriceBumpPercent)))
				min.Div(min, big.NewInt(100))
				if e.Price.Cmp(min) < 0 {
					p.noteRejectLocked(ErrReplaceUnder)
					return dewtypes.Hash{}, ErrReplaceUnder
				}
				// Same kind preferred; allow cross-kind RBF by price only.
				p.removeLocked(oldHash)
				replaced = true
			}
		}
	}

	// Per-sender limit (after potential replacement removed one).
	senderCount := 0
	if m, ok := p.bySender[e.From]; ok {
		senderCount = len(m)
	}
	if senderCount >= p.cfg.MaxPerSender {
		p.noteRejectLocked(ErrSenderLimit)
		return dewtypes.Hash{}, ErrSenderLimit
	}

	// Global limit: if full, try to evict a cheaper tx from another slot;
	// if none cheaper, reject.
	if len(p.byHash) >= p.cfg.MaxGlobal {
		if !p.evictCheaperThan(e.Price) {
			p.noteRejectLocked(ErrPoolFull)
			return dewtypes.Hash{}, ErrPoolFull
		}
	}

	p.byHash[e.Hash] = e
	if p.bySender[e.From] == nil {
		p.bySender[e.From] = make(map[uint64]dewtypes.Hash)
	}
	p.bySender[e.From][e.Nonce] = e.Hash
	p.admits++
	if replaced {
		p.replaces++
	}
	return e.Hash, nil
}

// evictCheaperThan removes one lowest-price entry strictly cheaper than price.
// Returns false if nothing was evicted.
func (p *Pool) evictCheaperThan(price *big.Int) bool {
	var worst *Entry
	for _, e := range p.byHash {
		if e.Price.Cmp(price) >= 0 {
			continue
		}
		if worst == nil || e.Price.Cmp(worst.Price) < 0 {
			worst = e
		}
	}
	if worst == nil {
		return false
	}
	p.removeLocked(worst.Hash)
	p.evictions++
	return true
}

// Pending returns a snapshot of all entries (unsorted).
func (p *Pool) Pending() []*Entry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*Entry, 0, len(p.byHash))
	for _, e := range p.byHash {
		out = append(out, e)
	}
	return out
}

// Stats returns (global, unique senders).
func (p *Pool) Stats() (global, senders int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.byHash), len(p.bySender)
}

// Telemetry returns a read-only snapshot for lab / RPC observability.
func (p *Pool) Telemetry() Telemetry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pendingEVM, pendingDew := 0, 0
	for _, e := range p.byHash {
		switch e.Kind {
		case KindEVM:
			pendingEVM++
		case KindDew:
			pendingDew++
		}
	}

	// Top senders by pending count (cap 16).
	type pair struct {
		from crypto.Address
		n    int
	}
	list := make([]pair, 0, len(p.bySender))
	for from, m := range p.bySender {
		list = append(list, pair{from: from, n: len(m)})
	}
	// Simple insertion sort by n desc (N is small, max pool size).
	for i := 1; i < len(list); i++ {
		j := i
		for j > 0 && list[j].n > list[j-1].n {
			list[j], list[j-1] = list[j-1], list[j]
			j--
		}
	}
	const topN = 16
	if len(list) > topN {
		list = list[:topN]
	}
	top := make([]SenderPending, len(list))
	for i, it := range list {
		top[i] = SenderPending{Address: it.from, Pending: it.n}
	}

	minGas := "0"
	if p.cfg.MinGasPriceWei != nil {
		minGas = p.cfg.MinGasPriceWei.String()
	}
	minTip := "0"
	if p.cfg.MinTipWei != nil {
		minTip = p.cfg.MinTipWei.String()
	}

	return Telemetry{
		Pending:          len(p.byHash),
		Senders:          len(p.bySender),
		PendingEVM:       pendingEVM,
		PendingDew:       pendingDew,
		MaxGlobal:        p.cfg.MaxGlobal,
		MaxPerSender:     p.cfg.MaxPerSender,
		MaxTxBytes:       p.cfg.MaxTxBytes,
		MinGasPriceWei:   minGas,
		MinTipWei:        minTip,
		MinDewFeeWei:     p.cfg.MinDewFeeWei,
		PriceBumpPercent: p.cfg.PriceBumpPercent,
		Admits:           p.admits,
		Replaces:         p.replaces,
		Evictions:        p.evictions,
		Rejects:          p.rejects,
		TopSenders:       top,
	}
}

func (p *Pool) noteReject(err error) {
	// Caller must hold p.mu when incrementing counters that race with Telemetry,
	// OR call noteRejectLocked. Pre-lock rejects use noteReject with its own lock.
	p.mu.Lock()
	defer p.mu.Unlock()
	p.noteRejectLocked(err)
}

func (p *Pool) noteRejectLocked(err error) {
	if err == nil {
		return
	}
	p.rejects.Total++
	switch {
	case errors.Is(err, ErrPoolFull):
		p.rejects.PoolFull++
	case errors.Is(err, ErrSenderLimit):
		p.rejects.SenderLimit++
	case errors.Is(err, ErrUnderpriced):
		p.rejects.Underpriced++
	case errors.Is(err, ErrReplaceUnder):
		p.rejects.ReplaceUnder++
	case errors.Is(err, ErrAlreadyKnown):
		p.rejects.AlreadyKnown++
	case errors.Is(err, ErrTxTooLarge):
		p.rejects.TxTooLarge++
	case errors.Is(err, ErrWrongChain):
		p.rejects.WrongChain++
	case errors.Is(err, ErrRBFDisabled):
		p.rejects.RBFDisabled++
	default:
		p.rejects.Invalid++
	}
}
