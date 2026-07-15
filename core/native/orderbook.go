// Package native hosts Dew-native execution paths (DewTx + staking + orderbook).
package native

import (
	"fmt"
	"math/big"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

// OrderbookModuleAddr is the system account holding orderbook storage (precompile 0x101).
var OrderbookModuleAddr = crypto.MustHexToAddress("0x0000000000000000000000000000000000000101")

// Order side and status constants.
const (
	OrderSideBuy  uint8 = 0
	OrderSideSell uint8 = 1

	OrderStatusOpen      uint8 = 0
	OrderStatusFilled    uint8 = 1
	OrderStatusCancelled uint8 = 2
)

// Storage slot layout (under OrderbookModuleAddr):
//
//	keccak256("dew/ob/v1/nextId")
//	keccak256("dew/ob/v1/order" || id8 || field)
//	keccak256("dew/ob/v1/openCount" || maker)
//	keccak256("dew/ob/v1/escrowQ" || maker)
//	keccak256("dew/ob/v1/escrowB" || maker || token)
//	keccak256("dew/ob/v1/openLen" || token || side)
//	keccak256("dew/ob/v1/openAt" || token || side || idx8)
const (
	obDomainNextID    = "dew/ob/v1/nextId"
	obDomainOrder     = "dew/ob/v1/order"
	obDomainOpenCount = "dew/ob/v1/openCount"
	obDomainEscrowQ   = "dew/ob/v1/escrowQ"
	obDomainEscrowB   = "dew/ob/v1/escrowB"
	obDomainOpenLen   = "dew/ob/v1/openLen"
	obDomainOpenAt    = "dew/ob/v1/openAt"

	obFieldHdr   byte = 0
	obFieldToken byte = 1
	obFieldPrice byte = 2
	obFieldOpen  byte = 3
	obFieldOrig  byte = 4
)

// Order is a resting limit order.
type Order struct {
	ID        uint64
	Maker     crypto.Address
	BaseToken crypto.Address
	Side      uint8
	PriceX18  *uint256.Int
	BaseOpen  *uint256.Int
	BaseOrig  *uint256.Int
	CreatedAt uint64
	Status    uint8
}

// Refund is quote/base returned on cancel (precompile transfers funds).
type Refund struct {
	Quote *uint256.Int
	Base  *uint256.Int
	Token crypto.Address
	Side  uint8
}

// OrderbookModule reads/writes orderbook state via flat StateDB storage.
type OrderbookModule struct {
	db *state.StateDB
}

// NewOrderbookModule binds to statedb.
func NewOrderbookModule(db *state.StateDB) *OrderbookModule {
	return &OrderbookModule{db: db}
}

func obSlot(parts ...[]byte) types.Hash {
	return types.BytesToHash(crypto.Keccak256(parts...))
}

func u64BE(v uint64) []byte {
	var be [8]byte
	for b := 0; b < 8; b++ {
		be[7-b] = byte(v >> (8 * b))
	}
	return be[:]
}

func oneE18() *uint256.Int {
	return uint256.NewInt(1e18)
}

// QuoteFloor returns floor(base * priceX18 / 1e18).
func QuoteFloor(base, priceX18 *uint256.Int) *uint256.Int {
	if base == nil || priceX18 == nil || base.IsZero() || priceX18.IsZero() {
		return uint256.NewInt(0)
	}
	// Use big.Int to avoid intermediate overflow concerns for large values.
	prod := new(big.Int).Mul(base.ToBig(), priceX18.ToBig())
	prod.Div(prod, oneE18().ToBig())
	out, _ := uint256.FromBig(prod)
	if out == nil {
		return uint256.NewInt(0)
	}
	return out
}

// QuoteLockCeil returns ceil(base * priceX18 / 1e18) for buy place escrow.
func QuoteLockCeil(base, priceX18 *uint256.Int) *uint256.Int {
	if base == nil || priceX18 == nil || base.IsZero() || priceX18.IsZero() {
		return uint256.NewInt(0)
	}
	prod := new(big.Int).Mul(base.ToBig(), priceX18.ToBig())
	den := oneE18().ToBig()
	// ceil = (prod + den - 1) / den
	prod.Add(prod, new(big.Int).Sub(den, big.NewInt(1)))
	prod.Div(prod, den)
	out, _ := uint256.FromBig(prod)
	if out == nil {
		return uint256.NewInt(0)
	}
	return out
}

func (m *OrderbookModule) nextIDSlot() types.Hash {
	return obSlot([]byte(obDomainNextID))
}

func (m *OrderbookModule) orderSlot(id uint64, field byte) types.Hash {
	return obSlot([]byte(obDomainOrder), u64BE(id), []byte{field})
}

func (m *OrderbookModule) openCountSlot(maker crypto.Address) types.Hash {
	return obSlot([]byte(obDomainOpenCount), maker.Bytes())
}

func (m *OrderbookModule) escrowQSlot(maker crypto.Address) types.Hash {
	return obSlot([]byte(obDomainEscrowQ), maker.Bytes())
}

func (m *OrderbookModule) escrowBSlot(maker, token crypto.Address) types.Hash {
	return obSlot([]byte(obDomainEscrowB), maker.Bytes(), token.Bytes())
}

func (m *OrderbookModule) openLenSlot(token crypto.Address, side uint8) types.Hash {
	return obSlot([]byte(obDomainOpenLen), token.Bytes(), []byte{side})
}

func (m *OrderbookModule) openAtSlot(token crypto.Address, side uint8, idx uint64) types.Hash {
	return obSlot([]byte(obDomainOpenAt), token.Bytes(), []byte{side}, u64BE(idx))
}

func (m *OrderbookModule) getU256(slot types.Hash) *uint256.Int {
	return hashToU256(m.db.GetState(OrderbookModuleAddr, slot))
}

func (m *OrderbookModule) setU256(slot types.Hash, v *uint256.Int) {
	m.db.SetState(OrderbookModuleAddr, slot, u256ToHash(v))
}

// OpenCount returns concurrent open orders for maker.
func (m *OrderbookModule) OpenCount(maker crypto.Address) uint64 {
	return m.getU256(m.openCountSlot(maker)).Uint64()
}

// EscrowQuote returns accounted quote DEW escrow for maker.
func (m *OrderbookModule) EscrowQuote(maker crypto.Address) *uint256.Int {
	return m.getU256(m.escrowQSlot(maker))
}

// EscrowBase returns accounted base token escrow for maker/token.
func (m *OrderbookModule) EscrowBase(maker, token crypto.Address) *uint256.Int {
	return m.getU256(m.escrowBSlot(maker, token))
}

func (m *OrderbookModule) nextOrderID() uint64 {
	cur := m.getU256(m.nextIDSlot()).Uint64()
	next := cur + 1
	m.setU256(m.nextIDSlot(), uint256.NewInt(next))
	return next
}

func (m *OrderbookModule) writeOrder(o Order) {
	// hdr: maker(20) || side(1) || status(1) || createdAt(8) packed in 32 bytes
	var hdr types.Hash
	copy(hdr[0:20], o.Maker[:])
	hdr[20] = o.Side
	hdr[21] = o.Status
	copy(hdr[24:32], u64BE(o.CreatedAt))
	m.db.SetState(OrderbookModuleAddr, m.orderSlot(o.ID, obFieldHdr), hdr)

	var tok types.Hash
	copy(tok[12:], o.BaseToken[:])
	m.db.SetState(OrderbookModuleAddr, m.orderSlot(o.ID, obFieldToken), tok)
	m.setU256(m.orderSlot(o.ID, obFieldPrice), o.PriceX18)
	m.setU256(m.orderSlot(o.ID, obFieldOpen), o.BaseOpen)
	m.setU256(m.orderSlot(o.ID, obFieldOrig), o.BaseOrig)
}

// GetOrder loads an order by id. ok is false if id was never assigned.
func (m *OrderbookModule) GetOrder(id uint64) (Order, bool) {
	if id == 0 {
		return Order{}, false
	}
	next := m.getU256(m.nextIDSlot()).Uint64()
	if id > next {
		return Order{}, false
	}
	hdr := m.db.GetState(OrderbookModuleAddr, m.orderSlot(id, obFieldHdr))
	// Unassigned ids below next still have zero hdr — treat maker zero + zero orig as missing
	// after we always write non-zero orig on place.
	tokH := m.db.GetState(OrderbookModuleAddr, m.orderSlot(id, obFieldToken))
	price := m.getU256(m.orderSlot(id, obFieldPrice))
	open := m.getU256(m.orderSlot(id, obFieldOpen))
	orig := m.getU256(m.orderSlot(id, obFieldOrig))
	if orig.IsZero() && open.IsZero() && price.IsZero() {
		// Could be a never-written gap; still return if maker non-zero from hdr
		var maker crypto.Address
		copy(maker[:], hdr[0:20])
		if maker == (crypto.Address{}) {
			return Order{}, false
		}
	}
	var maker crypto.Address
	copy(maker[:], hdr[0:20])
	var token crypto.Address
	copy(token[:], tokH[12:])
	createdAt := new(uint256.Int).SetBytes(hdr[24:32]).Uint64()
	return Order{
		ID:        id,
		Maker:     maker,
		BaseToken: token,
		Side:      hdr[20],
		PriceX18:  price,
		BaseOpen:  open,
		BaseOrig:  orig,
		CreatedAt: createdAt,
		Status:    hdr[21],
	}, true
}

func (m *OrderbookModule) addOpenIndex(token crypto.Address, side uint8, id uint64) {
	lenSlot := m.openLenSlot(token, side)
	n := m.getU256(lenSlot).Uint64()
	m.setU256(m.openAtSlot(token, side, n), uint256.NewInt(id))
	m.setU256(lenSlot, uint256.NewInt(n+1))
}

func (m *OrderbookModule) removeOpenIndex(token crypto.Address, side uint8, id uint64) {
	lenSlot := m.openLenSlot(token, side)
	n := m.getU256(lenSlot).Uint64()
	for i := uint64(0); i < n; i++ {
		slot := m.openAtSlot(token, side, i)
		if m.getU256(slot).Uint64() == id {
			// swap-remove with last
			if i != n-1 {
				last := m.getU256(m.openAtSlot(token, side, n-1))
				m.setU256(slot, last)
			}
			m.setU256(m.openAtSlot(token, side, n-1), uint256.NewInt(0))
			m.setU256(lenSlot, uint256.NewInt(n-1))
			return
		}
	}
}

func (m *OrderbookModule) incOpenCount(maker crypto.Address) error {
	n := m.OpenCount(maker)
	if n >= params.OrderbookMaxOpenPerMaker {
		return fmt.Errorf("orderbook: max open orders per maker (%d)", params.OrderbookMaxOpenPerMaker)
	}
	m.setU256(m.openCountSlot(maker), uint256.NewInt(n+1))
	return nil
}

func (m *OrderbookModule) decOpenCount(maker crypto.Address) {
	n := m.OpenCount(maker)
	if n == 0 {
		return
	}
	m.setU256(m.openCountSlot(maker), uint256.NewInt(n-1))
}

func (m *OrderbookModule) addEscrowQ(maker crypto.Address, amt *uint256.Int) {
	cur := m.EscrowQuote(maker)
	m.setU256(m.escrowQSlot(maker), new(uint256.Int).Add(cur, amt))
}

func (m *OrderbookModule) subEscrowQ(maker crypto.Address, amt *uint256.Int) error {
	cur := m.EscrowQuote(maker)
	if cur.Cmp(amt) < 0 {
		return fmt.Errorf("orderbook: escrow quote underflow")
	}
	m.setU256(m.escrowQSlot(maker), new(uint256.Int).Sub(cur, amt))
	return nil
}

func (m *OrderbookModule) addEscrowB(maker, token crypto.Address, amt *uint256.Int) {
	cur := m.EscrowBase(maker, token)
	m.setU256(m.escrowBSlot(maker, token), new(uint256.Int).Add(cur, amt))
}

func (m *OrderbookModule) subEscrowB(maker, token crypto.Address, amt *uint256.Int) error {
	cur := m.EscrowBase(maker, token)
	if cur.Cmp(amt) < 0 {
		return fmt.Errorf("orderbook: escrow base underflow")
	}
	m.setU256(m.escrowBSlot(maker, token), new(uint256.Int).Sub(cur, amt))
	return nil
}

func (m *OrderbookModule) validatePlace(maker crypto.Address, price, baseAmt *uint256.Int) error {
	if maker == (crypto.Address{}) {
		return fmt.Errorf("orderbook: zero maker")
	}
	if price == nil || price.IsZero() {
		return fmt.Errorf("orderbook: zero price")
	}
	if baseAmt == nil || baseAmt.IsZero() {
		return fmt.Errorf("orderbook: zero base amount")
	}
	return m.incOpenCount(maker)
}

// PlaceBuy records a buy order and increases quote escrow accounting by quoteLock (ceil).
// Precompile must have received CALLVALUE == quoteLock before calling.
func (m *OrderbookModule) PlaceBuy(maker, token crypto.Address, price, baseAmt *uint256.Int, blockNum uint64) (id uint64, quoteLock *uint256.Int, err error) {
	if err = m.validatePlace(maker, price, baseAmt); err != nil {
		return 0, nil, err
	}
	if token == (crypto.Address{}) {
		m.decOpenCount(maker)
		return 0, nil, fmt.Errorf("orderbook: zero base token")
	}
	quoteLock = QuoteLockCeil(baseAmt, price)
	if quoteLock.IsZero() {
		m.decOpenCount(maker)
		return 0, nil, fmt.Errorf("orderbook: quote lock is zero")
	}
	id = m.nextOrderID()
	o := Order{
		ID:        id,
		Maker:     maker,
		BaseToken: token,
		Side:      OrderSideBuy,
		PriceX18:  new(uint256.Int).Set(price),
		BaseOpen:  new(uint256.Int).Set(baseAmt),
		BaseOrig:  new(uint256.Int).Set(baseAmt),
		CreatedAt: blockNum,
		Status:    OrderStatusOpen,
	}
	m.writeOrder(o)
	m.addEscrowQ(maker, quoteLock)
	m.addOpenIndex(token, OrderSideBuy, id)
	return id, quoteLock, nil
}

// PlaceSell records a sell order and increases base escrow accounting.
// Precompile must have transferFrom'd baseAmt to the module first.
func (m *OrderbookModule) PlaceSell(maker, token crypto.Address, price, baseAmt *uint256.Int, blockNum uint64) (id uint64, err error) {
	if err = m.validatePlace(maker, price, baseAmt); err != nil {
		return 0, err
	}
	if token == (crypto.Address{}) {
		m.decOpenCount(maker)
		return 0, fmt.Errorf("orderbook: zero base token")
	}
	id = m.nextOrderID()
	o := Order{
		ID:        id,
		Maker:     maker,
		BaseToken: token,
		Side:      OrderSideSell,
		PriceX18:  new(uint256.Int).Set(price),
		BaseOpen:  new(uint256.Int).Set(baseAmt),
		BaseOrig:  new(uint256.Int).Set(baseAmt),
		CreatedAt: blockNum,
		Status:    OrderStatusOpen,
	}
	m.writeOrder(o)
	m.addEscrowB(maker, token, baseAmt)
	m.addOpenIndex(token, OrderSideSell, id)
	return id, nil
}

// Cancel cancels an open order. Only the maker may cancel.
// Returns refund amounts; precompile transfers funds out of 0x101.
func (m *OrderbookModule) Cancel(actor crypto.Address, id uint64) (Refund, error) {
	o, ok := m.GetOrder(id)
	if !ok {
		return Refund{}, fmt.Errorf("orderbook: order not found")
	}
	if o.Status != OrderStatusOpen {
		return Refund{}, fmt.Errorf("orderbook: order not open")
	}
	if o.Maker != actor {
		return Refund{}, fmt.Errorf("orderbook: not maker")
	}
	refund := Refund{
		Quote: uint256.NewInt(0),
		Base:  uint256.NewInt(0),
		Token: o.BaseToken,
		Side:  o.Side,
	}
	if o.Side == OrderSideBuy {
		// Remaining locked quote proportional by ceil on remaining base, but we
		// locked ceil(orig*price). Safer: refund QuoteFloor remaining would under-refund.
		// Lock: refund ceil(baseOpen * price) capped by escrow; use same ceil as place
		// on remaining open so full cancel of untouched order returns original lock.
		q := QuoteLockCeil(o.BaseOpen, o.PriceX18)
		// Cap by current escrow in case of fill dust accounting.
		esc := m.EscrowQuote(o.Maker)
		if q.Cmp(esc) > 0 {
			q = new(uint256.Int).Set(esc)
		}
		if err := m.subEscrowQ(o.Maker, q); err != nil {
			return Refund{}, err
		}
		refund.Quote = q
	} else {
		if err := m.subEscrowB(o.Maker, o.BaseToken, o.BaseOpen); err != nil {
			return Refund{}, err
		}
		refund.Base = new(uint256.Int).Set(o.BaseOpen)
	}
	o.Status = OrderStatusCancelled
	o.BaseOpen = uint256.NewInt(0)
	m.writeOrder(o)
	m.decOpenCount(o.Maker)
	m.removeOpenIndex(o.BaseToken, o.Side, id)
	return refund, nil
}

// Fill executes against a resting order (any taker). Returns base filled and quote paid.
// Precompile moves assets; module only adjusts accounting.
//
// Buy order (maker buys base): taker sells base, receives quoteFloor(baseFilled).
// Sell order (maker sells base): taker buys base, pays quoteFloor(baseFilled).
func (m *OrderbookModule) Fill(taker crypto.Address, id uint64, maxBase *uint256.Int) (baseFilled, quotePaid *uint256.Int, err error) {
	if taker == (crypto.Address{}) {
		return nil, nil, fmt.Errorf("orderbook: zero taker")
	}
	if maxBase == nil || maxBase.IsZero() {
		return nil, nil, fmt.Errorf("orderbook: zero fill amount")
	}
	o, ok := m.GetOrder(id)
	if !ok {
		return nil, nil, fmt.Errorf("orderbook: order not found")
	}
	if o.Status != OrderStatusOpen {
		return nil, nil, fmt.Errorf("orderbook: order not open")
	}
	baseFilled = new(uint256.Int).Set(maxBase)
	if baseFilled.Cmp(o.BaseOpen) > 0 {
		baseFilled.Set(o.BaseOpen)
	}
	quotePaid = QuoteFloor(baseFilled, o.PriceX18)
	// Zero quote is allowed (dust price) but still reduce base; rare.

	if o.Side == OrderSideBuy {
		// Maker pays quote to taker from escrow. Use floor for this fill; on full
		// fill of remaining, release any residual ceil-lock dust to keep escrow
		// solvent: remaining lock after partial uses ceil on leftover.
		// Simple approach: subtract quotePaid; if order fully filled, also release
		// leftover escrow that was reserved for this order's remaining size.
		if err = m.subEscrowQ(o.Maker, quotePaid); err != nil {
			return nil, nil, err
		}
		remaining := new(uint256.Int).Sub(o.BaseOpen, baseFilled)
		if remaining.IsZero() {
			// Release residual lock: we originally locked ceil(orig) and have been
			// paying floor fills. Residual = min(escrow, ceil(0)=0) — compute
			// leftover as: nothing extra if we only track floor. For full fill of
			// original: sum of floor fills may be < ceil lock. Release dust so
			// maker does not leave stranded escrow without an order.
			// Approximate residual for this order: QuoteLockCeil(baseFilled was full open)
			// was locked at place as ceil(orig). Sum of all floor(fills) + cancel path.
			// On final fill, residual = Escrow attributable is hard; release
			// QuoteLockCeil(o.BaseOpen before fill) - quotePaid for this slice only when
			// remaining becomes 0: residual = ceil(baseOpen_before) - floor(baseOpen_before)
			// which is 0 or 1 typically.
			beforeOpen := new(uint256.Int).Set(o.BaseOpen)
			ceilRem := QuoteLockCeil(beforeOpen, o.PriceX18)
			if ceilRem.Cmp(quotePaid) > 0 {
				dust := new(uint256.Int).Sub(ceilRem, quotePaid)
				// Dust stays with maker as residual escrow — on full fill return dust
				// to maker by reducing escrow (precompile should refund dust).
				// Module: leave dust in escrowQ so precompile can refund maker on full fill.
				// Actually plan says dust stays in module. Leave escrow as-is after sub quotePaid.
				_ = dust
			}
			o.Status = OrderStatusFilled
			m.decOpenCount(o.Maker)
			m.removeOpenIndex(o.BaseToken, o.Side, id)
		}
		o.BaseOpen = remaining
	} else {
		// Sell: reduce base escrow
		if err = m.subEscrowB(o.Maker, o.BaseToken, baseFilled); err != nil {
			return nil, nil, err
		}
		remaining := new(uint256.Int).Sub(o.BaseOpen, baseFilled)
		if remaining.IsZero() {
			o.Status = OrderStatusFilled
			m.decOpenCount(o.Maker)
			m.removeOpenIndex(o.BaseToken, o.Side, id)
		}
		o.BaseOpen = remaining
	}
	m.writeOrder(o)
	return baseFilled, quotePaid, nil
}

// BestBid returns highest price open buy for token (ties: lowest id).
func (m *OrderbookModule) BestBid(token crypto.Address) (price *uint256.Int, id uint64, ok bool) {
	return m.best(token, OrderSideBuy, true)
}

// BestAsk returns lowest price open sell for token (ties: lowest id).
func (m *OrderbookModule) BestAsk(token crypto.Address) (price *uint256.Int, id uint64, ok bool) {
	return m.best(token, OrderSideSell, false)
}

func (m *OrderbookModule) best(token crypto.Address, side uint8, preferHigh bool) (price *uint256.Int, id uint64, ok bool) {
	n := m.getU256(m.openLenSlot(token, side)).Uint64()
	var bestPrice *uint256.Int
	var bestID uint64
	for i := uint64(0); i < n; i++ {
		oid := m.getU256(m.openAtSlot(token, side, i)).Uint64()
		o, found := m.GetOrder(oid)
		if !found || o.Status != OrderStatusOpen {
			continue
		}
		if bestPrice == nil {
			bestPrice = new(uint256.Int).Set(o.PriceX18)
			bestID = oid
			continue
		}
		cmp := o.PriceX18.Cmp(bestPrice)
		if preferHigh {
			if cmp > 0 || (cmp == 0 && oid < bestID) {
				bestPrice.Set(o.PriceX18)
				bestID = oid
			}
		} else {
			if cmp < 0 || (cmp == 0 && oid < bestID) {
				bestPrice.Set(o.PriceX18)
				bestID = oid
			}
		}
	}
	if bestPrice == nil {
		return uint256.NewInt(0), 0, false
	}
	return bestPrice, bestID, true
}
