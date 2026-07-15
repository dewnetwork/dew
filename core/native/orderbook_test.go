package native

import (
	"testing"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
	"github.com/dewnetwork/dew/params"
)

func testOB(t *testing.T) (*OrderbookModule, *state.StateDB) {
	t.Helper()
	s := state.New(db.OpenTest(t))
	return NewOrderbookModule(s), s
}

func TestOrderbook_PlaceBuyLocksQuoteAndCancelRefunds(t *testing.T) {
	m, _ := testOB(t)
	maker := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	token := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	price := uint256.NewInt(2e18) // 2 quote per 1 base (1e18 units)
	baseAmt := uint256.NewInt(1e18)
	// ceil lock = 2e18
	id, quoteLock, err := m.PlaceBuy(maker, token, price, baseAmt, 10)
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("id %d", id)
	}
	if quoteLock.Cmp(uint256.NewInt(2e18)) != 0 {
		t.Fatalf("lock %s", quoteLock)
	}
	if m.EscrowQuote(maker).Cmp(quoteLock) != 0 {
		t.Fatalf("escrow quote")
	}
	if m.OpenCount(maker) != 1 {
		t.Fatal("open count")
	}
	refund, err := m.Cancel(maker, id)
	if err != nil {
		t.Fatal(err)
	}
	if refund.Quote.Cmp(quoteLock) != 0 || !refund.Base.IsZero() {
		t.Fatalf("refund %+v", refund)
	}
	if !m.EscrowQuote(maker).IsZero() || m.OpenCount(maker) != 0 {
		t.Fatal("after cancel")
	}
	if _, err := m.Cancel(maker, id); err == nil {
		t.Fatal("double cancel")
	}
}

func TestOrderbook_PlaceSellAndPartialFill(t *testing.T) {
	m, _ := testOB(t)
	maker := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	taker := crypto.MustHexToAddress("0x00000000000000000000000000000000000000cc")
	token := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	price := uint256.NewInt(1e18)
	baseAmt := uint256.NewInt(10)
	id, err := m.PlaceSell(maker, token, price, baseAmt, 5)
	if err != nil {
		t.Fatal(err)
	}
	// fill 4 base → quote floor = 4
	baseFilled, quotePaid, err := m.Fill(taker, id, uint256.NewInt(4))
	if err != nil {
		t.Fatal(err)
	}
	if baseFilled.Uint64() != 4 || quotePaid.Uint64() != 4 {
		t.Fatalf("filled %s quote %s", baseFilled, quotePaid)
	}
	o, ok := m.GetOrder(id)
	if !ok {
		t.Fatal("order missing")
	}
	if o.BaseOpen.Uint64() != 6 || o.Status != OrderStatusOpen {
		t.Fatalf("order %+v", o)
	}
	// fill rest
	_, _, err = m.Fill(taker, id, uint256.NewInt(100))
	if err != nil {
		t.Fatal(err)
	}
	o, ok = m.GetOrder(id)
	if !ok || o.Status != OrderStatusFilled || !o.BaseOpen.IsZero() {
		t.Fatalf("filled order %+v ok=%v", o, ok)
	}
}

func TestOrderbook_MaxOpenPerMaker(t *testing.T) {
	m, _ := testOB(t)
	maker := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	token := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	price := uint256.NewInt(1e18)
	for i := uint64(0); i < params.OrderbookMaxOpenPerMaker; i++ {
		_, _, err := m.PlaceBuy(maker, token, price, uint256.NewInt(1), i+1)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := m.PlaceBuy(maker, token, price, uint256.NewInt(1), 999); err == nil {
		t.Fatal("expected max open")
	}
}

func TestOrderbook_QuoteCeilFloor(t *testing.T) {
	price := uint256.NewInt(1e18)
	if QuoteLockCeil(uint256.NewInt(3), price).Uint64() != 3 {
		t.Fatal("ceil")
	}
	if QuoteFloor(uint256.NewInt(2), price).Uint64() != 2 {
		t.Fatal("floor")
	}
	// odd: base=1, price=1 (1 wei per 1e18 base) → floor 0
	if !QuoteFloor(uint256.NewInt(1), uint256.NewInt(1)).IsZero() {
		t.Fatal("tiny floor")
	}
}

func TestOrderbook_BestBidAsk(t *testing.T) {
	m, _ := testOB(t)
	maker := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	token := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	_, _, _ = m.PlaceBuy(maker, token, uint256.NewInt(100), uint256.NewInt(1e18), 1)
	_, _, _ = m.PlaceBuy(maker, token, uint256.NewInt(200), uint256.NewInt(1e18), 2)
	_, _ = m.PlaceSell(maker, token, uint256.NewInt(300), uint256.NewInt(1e18), 3)
	_, _ = m.PlaceSell(maker, token, uint256.NewInt(250), uint256.NewInt(1e18), 4)
	bp, _, ok := m.BestBid(token)
	if !ok || bp.Uint64() != 200 {
		t.Fatalf("bid %v %s", ok, bp)
	}
	ap, _, ok := m.BestAsk(token)
	if !ok || ap.Uint64() != 250 {
		t.Fatalf("ask %v %s", ok, ap)
	}
}

func TestOrderbook_FillBuyPaysTakerQuote(t *testing.T) {
	m, _ := testOB(t)
	maker := crypto.MustHexToAddress("0x00000000000000000000000000000000000000aa")
	taker := crypto.MustHexToAddress("0x00000000000000000000000000000000000000cc")
	token := crypto.MustHexToAddress("0x00000000000000000000000000000000000000bb")
	price := uint256.NewInt(2e18)
	baseAmt := uint256.NewInt(1e18)
	id, lock, err := m.PlaceBuy(maker, token, price, baseAmt, 1)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Cmp(uint256.NewInt(2e18)) != 0 {
		t.Fatalf("lock %s", lock)
	}
	baseFilled, quotePaid, err := m.Fill(taker, id, uint256.NewInt(5e17)) // half base
	if err != nil {
		t.Fatal(err)
	}
	// floor(0.5e18 * 2e18 / 1e18) = 1e18
	if baseFilled.Cmp(uint256.NewInt(5e17)) != 0 || quotePaid.Cmp(uint256.NewInt(1e18)) != 0 {
		t.Fatalf("filled %s quote %s", baseFilled, quotePaid)
	}
	if m.EscrowQuote(maker).Cmp(uint256.NewInt(1e18)) != 0 {
		t.Fatalf("remaining escrow %s", m.EscrowQuote(maker))
	}
}
