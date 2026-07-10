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

// Pool is a unified mempool for EVM and DewTx.
type Pool struct {
	cfg Config

	mu      sync.RWMutex
	byHash  map[dewtypes.Hash]*Entry
	// bySender maps sender → nonce → hash
	bySender map[crypto.Address]map[uint64]dewtypes.Hash
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
		return dewtypes.Hash{}, ErrInvalidTx
	}
	if len(raw) == 0 {
		var err error
		raw, err = tx.MarshalBinary()
		if err != nil {
			return dewtypes.Hash{}, fmt.Errorf("%w: marshal: %v", ErrInvalidTx, err)
		}
	}
	if len(raw) > p.cfg.MaxTxBytes {
		return dewtypes.Hash{}, ErrTxTooLarge
	}
	if tx.ChainId() != nil && tx.ChainId().Sign() != 0 && chainID != nil && tx.ChainId().Cmp(chainID) != 0 {
		return dewtypes.Hash{}, ErrWrongChain
	}

	price, tip, err := p.evmPrice(tx)
	if err != nil {
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
		return dewtypes.Hash{}, ErrInvalidTx
	}
	if len(raw) == 0 {
		var err error
		raw, err = tx.MarshalBinary()
		if err != nil {
			return dewtypes.Hash{}, fmt.Errorf("%w: marshal: %v", ErrInvalidTx, err)
		}
	}
	if len(raw) > p.cfg.MaxTxBytes {
		return dewtypes.Hash{}, ErrTxTooLarge
	}
	if tx.ChainID == nil || (chainID != nil && tx.ChainID.Cmp(chainID) != 0) {
		return dewtypes.Hash{}, ErrWrongChain
	}

	fee := tx.Fee
	if fee == 0 {
		// Protocol substitutes default; admission still requires min floor.
		fee = p.cfg.MinDewFeeWei
	}
	if fee < p.cfg.MinDewFeeWei {
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
		return e.Hash, ErrAlreadyKnown
	}

	// Same sender+nonce → replace-by-fee or reject.
	if m, ok := p.bySender[e.From]; ok {
		if oldHash, exists := m[e.Nonce]; exists {
			old := p.byHash[oldHash]
			if old == nil {
				delete(m, e.Nonce)
			} else {
				if p.cfg.PriceBumpPercent == 0 {
					return dewtypes.Hash{}, ErrRBFDisabled
				}
				// Require price * (100+bump)/100
				min := new(big.Int).Mul(old.Price, big.NewInt(int64(100+p.cfg.PriceBumpPercent)))
				min.Div(min, big.NewInt(100))
				if e.Price.Cmp(min) < 0 {
					return dewtypes.Hash{}, ErrReplaceUnder
				}
				// Same kind preferred; allow cross-kind RBF by price only.
				p.removeLocked(oldHash)
			}
		}
	}

	// Per-sender limit (after potential replacement removed one).
	senderCount := 0
	if m, ok := p.bySender[e.From]; ok {
		senderCount = len(m)
	}
	if senderCount >= p.cfg.MaxPerSender {
		return dewtypes.Hash{}, ErrSenderLimit
	}

	// Global limit: if full, try to evict a cheaper tx from another slot;
	// if none cheaper, reject.
	if len(p.byHash) >= p.cfg.MaxGlobal {
		if !p.evictCheaperThan(e.Price) {
			return dewtypes.Hash{}, ErrPoolFull
		}
	}

	p.byHash[e.Hash] = e
	if p.bySender[e.From] == nil {
		p.bySender[e.From] = make(map[uint64]dewtypes.Hash)
	}
	p.bySender[e.From][e.Nonce] = e.Hash
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
