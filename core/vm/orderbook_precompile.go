package vm

import (
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

// Orderbook method bytes (fail-closed fixed layout; not full Solidity ABI).
const (
	// ObMethodPlace — side u8 || baseToken 20 || priceX18 u256 || baseAmount u256.
	ObMethodPlace byte = 0x00
	// ObMethodCancel — orderId u256.
	ObMethodCancel byte = 0x01
	// ObMethodFill — orderId u256 || baseAmount u256.
	ObMethodFill byte = 0x02
	// ObMethodGetOrder — orderId u256.
	ObMethodGetOrder byte = 0x03
	// ObMethodBestBid — baseToken 20.
	ObMethodBestBid byte = 0x04
	// ObMethodBestAsk — baseToken 20.
	ObMethodBestAsk byte = 0x05
	// ObMethodGetEscrow — maker 20 || baseToken 20.
	ObMethodGetEscrow byte = 0x06
)

// ERC-20 selectors.
var (
	selTransferFrom = ethcommon.Hex2Bytes("23b872dd")
	selTransfer     = ethcommon.Hex2Bytes("a9059cbb")
)

const erc20CallGas uint64 = 100_000

// orderbookPrecompile implements 0x101 (limit orderbook).
type orderbookPrecompile struct {
	statedb  *state.StateDB
	self     crypto.Address
	origin   crypto.Address
	evm      *ethvm.EVM
	valueCtx *stakeValueCtx
	blockNum uint64
	enabled  bool
}

func (p *orderbookPrecompile) RequiredGas(input []byte) uint64 {
	if len(input) == 0 {
		return params.OrderbookGasQuery
	}
	switch input[0] {
	case ObMethodPlace:
		return params.OrderbookGasPlace
	case ObMethodCancel:
		return params.OrderbookGasCancel
	case ObMethodFill:
		return params.OrderbookGasFill
	default:
		return params.OrderbookGasQuery
	}
}

func (p *orderbookPrecompile) Name() string { return "DEW_NATIVE_SWAP" }

func (p *orderbookPrecompile) Run(input []byte) ([]byte, error) {
	if !p.enabled {
		return nil, fmt.Errorf("native swap: not enabled (set EnableNativeSwap)")
	}
	if len(input) < 1 {
		return nil, fmt.Errorf("native swap: empty input")
	}
	mod := native.NewOrderbookModule(p.statedb)
	switch input[0] {
	case ObMethodPlace:
		return p.place(mod, input)
	case ObMethodCancel:
		return p.cancel(mod, input)
	case ObMethodFill:
		return p.fill(mod, input)
	case ObMethodGetOrder:
		return p.getOrder(mod, input)
	case ObMethodBestBid:
		return p.best(mod, input, true)
	case ObMethodBestAsk:
		return p.best(mod, input, false)
	case ObMethodGetEscrow:
		return p.getEscrow(mod, input)
	default:
		return nil, fmt.Errorf("method")
	}
}

func (p *orderbookPrecompile) actor() crypto.Address {
	return p.origin
}

func (p *orderbookPrecompile) depositValue() *uint256.Int {
	if p.valueCtx != nil && p.valueCtx.amount != nil {
		return new(uint256.Int).Set(p.valueCtx.amount)
	}
	return uint256.NewInt(0)
}

func (p *orderbookPrecompile) consumeDeposit() {
	if p.valueCtx != nil {
		p.valueCtx.amount = uint256.NewInt(0)
	}
}

func (p *orderbookPrecompile) place(mod *native.OrderbookModule, input []byte) ([]byte, error) {
	// 1 + 1 + 20 + 32 + 32 = 86
	if len(input) != 86 {
		return nil, fmt.Errorf("native swap: place input length")
	}
	side := input[1]
	var token crypto.Address
	copy(token[:], input[2:22])
	price := new(uint256.Int).SetBytes(input[22:54])
	baseAmt := new(uint256.Int).SetBytes(input[54:86])
	actor := p.actor()

	switch side {
	case native.OrderSideBuy:
		quoteLock := native.QuoteLockCeil(baseAmt, price)
		val := p.depositValue()
		if val.Cmp(quoteLock) != 0 {
			return nil, fmt.Errorf("native swap: place buy requires CALLVALUE == quoteLock (%s got %s)", quoteLock, val)
		}
		id, lock, err := mod.PlaceBuy(actor, token, price, baseAmt, p.blockNum)
		if err != nil {
			return nil, err
		}
		if lock.Cmp(quoteLock) != 0 {
			return nil, fmt.Errorf("native swap: quote lock mismatch")
		}
		p.consumeDeposit()
		return u256Pad(uint256.NewInt(id)), nil

	case native.OrderSideSell:
		if !p.depositValue().IsZero() {
			return nil, fmt.Errorf("native swap: place sell requires zero CALLVALUE")
		}
		if err := p.erc20TransferFrom(token, actor, p.self, baseAmt); err != nil {
			return nil, err
		}
		id, err := mod.PlaceSell(actor, token, price, baseAmt, p.blockNum)
		if err != nil {
			return nil, err
		}
		return u256Pad(uint256.NewInt(id)), nil

	default:
		return nil, fmt.Errorf("native swap: invalid side")
	}
}

func (p *orderbookPrecompile) cancel(mod *native.OrderbookModule, input []byte) ([]byte, error) {
	if len(input) != 1+32 {
		return nil, fmt.Errorf("native swap: cancel needs orderId")
	}
	id := new(uint256.Int).SetBytes(input[1:33]).Uint64()
	refund, err := mod.Cancel(p.actor(), id)
	if err != nil {
		return nil, err
	}
	if refund.Quote != nil && !refund.Quote.IsZero() {
		modBal := p.statedb.GetBalance(p.self)
		if modBal.Cmp(refund.Quote) < 0 {
			return nil, fmt.Errorf("native swap: quote escrow insolvent")
		}
		p.statedb.SubBalance(p.self, refund.Quote)
		p.statedb.AddBalancePrev(p.actor(), refund.Quote)
	}
	if refund.Base != nil && !refund.Base.IsZero() {
		if err := p.erc20Transfer(refund.Token, p.actor(), refund.Base); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (p *orderbookPrecompile) fill(mod *native.OrderbookModule, input []byte) ([]byte, error) {
	if len(input) != 1+32+32 {
		return nil, fmt.Errorf("native swap: fill needs orderId + baseAmount")
	}
	id := new(uint256.Int).SetBytes(input[1:33]).Uint64()
	maxBase := new(uint256.Int).SetBytes(input[33:65])
	o, ok := mod.GetOrder(id)
	if !ok {
		return nil, fmt.Errorf("orderbook: order not found")
	}
	if o.Status != native.OrderStatusOpen {
		return nil, fmt.Errorf("orderbook: order not open")
	}

	// Pre-compute intended fill size for value checks (module will clamp).
	fillBase := new(uint256.Int).Set(maxBase)
	if fillBase.Cmp(o.BaseOpen) > 0 {
		fillBase.Set(o.BaseOpen)
	}
	quoteNeed := native.QuoteFloor(fillBase, o.PriceX18)
	taker := p.actor()

	if o.Side == native.OrderSideSell {
		// Taker buys base with DEW.
		val := p.depositValue()
		if val.Cmp(quoteNeed) < 0 {
			return nil, fmt.Errorf("native swap: fill sell needs CALLVALUE >= %s (got %s)", quoteNeed, val)
		}
		baseFilled, quotePaid, err := mod.Fill(taker, id, maxBase)
		if err != nil {
			return nil, err
		}
		// Transfer base to taker.
		if err := p.erc20Transfer(o.BaseToken, taker, baseFilled); err != nil {
			return nil, err
		}
		// Quote stays at 0x101 as maker proceeds (accounted via sell path: maker receives
		// by cancel? Design: on sell fill, taker pays quote, maker should receive quote.
		// Move quotePaid from module to maker.
		if !quotePaid.IsZero() {
			modBal := p.statedb.GetBalance(p.self)
			if modBal.Cmp(quotePaid) < 0 {
				return nil, fmt.Errorf("native swap: quote insolvent on fill")
			}
			p.statedb.SubBalance(p.self, quotePaid)
			p.statedb.AddBalancePrev(o.Maker, quotePaid)
		}
		// Refund excess CALLVALUE to taker.
		excess := new(uint256.Int).Sub(val, quotePaid)
		if excess.Sign() > 0 {
			modBal := p.statedb.GetBalance(p.self)
			if modBal.Cmp(excess) < 0 {
				return nil, fmt.Errorf("native swap: excess refund insolvent")
			}
			p.statedb.SubBalance(p.self, excess)
			p.statedb.AddBalancePrev(taker, excess)
		}
		p.consumeDeposit()
		return packFillOut(baseFilled, quotePaid), nil
	}

	// Buy order: taker sells base, receives quote.
	if !p.depositValue().IsZero() {
		return nil, fmt.Errorf("native swap: fill buy requires zero CALLVALUE")
	}
	if err := p.erc20TransferFrom(o.BaseToken, taker, p.self, fillBase); err != nil {
		return nil, err
	}
	baseFilled, quotePaid, err := mod.Fill(taker, id, maxBase)
	if err != nil {
		return nil, err
	}
	// If module filled less than pull, return excess base (should not happen if clamp matched).
	if baseFilled.Cmp(fillBase) < 0 {
		diff := new(uint256.Int).Sub(fillBase, baseFilled)
		if err := p.erc20Transfer(o.BaseToken, taker, diff); err != nil {
			return nil, err
		}
	}
	// Base stays at 0x101 for maker — transfer base to maker.
	if !baseFilled.IsZero() {
		if err := p.erc20Transfer(o.BaseToken, o.Maker, baseFilled); err != nil {
			return nil, err
		}
	}
	// Pay quote to taker from module DEW balance.
	if !quotePaid.IsZero() {
		modBal := p.statedb.GetBalance(p.self)
		if modBal.Cmp(quotePaid) < 0 {
			return nil, fmt.Errorf("native swap: quote insolvent on buy fill")
		}
		p.statedb.SubBalance(p.self, quotePaid)
		p.statedb.AddBalancePrev(taker, quotePaid)
	}
	return packFillOut(baseFilled, quotePaid), nil
}

func packFillOut(baseFilled, quotePaid *uint256.Int) []byte {
	out := make([]byte, 64)
	copy(out[0:32], u256Pad(baseFilled))
	copy(out[32:64], u256Pad(quotePaid))
	return out
}

func (p *orderbookPrecompile) getOrder(mod *native.OrderbookModule, input []byte) ([]byte, error) {
	if len(input) != 1+32 {
		return nil, fmt.Errorf("native swap: getOrder needs orderId")
	}
	id := new(uint256.Int).SetBytes(input[1:33]).Uint64()
	o, ok := mod.GetOrder(id)
	if !ok {
		return []byte{}, nil
	}
	// Packed: id||maker||token||side||price||baseOpen||baseOrig||createdAt||status (9×32)
	out := make([]byte, 9*32)
	copy(out[0:32], u256Pad(uint256.NewInt(o.ID)))
	copy(out[32:64], ethcommon.LeftPadBytes(o.Maker.Bytes(), 32))
	copy(out[64:96], ethcommon.LeftPadBytes(o.BaseToken.Bytes(), 32))
	copy(out[96:128], u256Pad(uint256.NewInt(uint64(o.Side))))
	copy(out[128:160], u256Pad(o.PriceX18))
	copy(out[160:192], u256Pad(o.BaseOpen))
	copy(out[192:224], u256Pad(o.BaseOrig))
	copy(out[224:256], u256Pad(uint256.NewInt(o.CreatedAt)))
	copy(out[256:288], u256Pad(uint256.NewInt(uint64(o.Status))))
	return out, nil
}

func (p *orderbookPrecompile) best(mod *native.OrderbookModule, input []byte, bid bool) ([]byte, error) {
	if len(input) != 1+20 {
		return nil, fmt.Errorf("native swap: best needs baseToken")
	}
	var token crypto.Address
	copy(token[:], input[1:21])
	var price *uint256.Int
	var id uint64
	var ok bool
	if bid {
		price, id, ok = mod.BestBid(token)
	} else {
		price, id, ok = mod.BestAsk(token)
	}
	out := make([]byte, 64)
	if !ok {
		return out, nil
	}
	copy(out[0:32], u256Pad(price))
	copy(out[32:64], u256Pad(uint256.NewInt(id)))
	return out, nil
}

func (p *orderbookPrecompile) getEscrow(mod *native.OrderbookModule, input []byte) ([]byte, error) {
	if len(input) != 1+20+20 {
		return nil, fmt.Errorf("native swap: getEscrow needs maker + baseToken")
	}
	var maker, token crypto.Address
	copy(maker[:], input[1:21])
	copy(token[:], input[21:41])
	base := mod.EscrowBase(maker, token)
	quote := mod.EscrowQuote(maker)
	out := make([]byte, 64)
	copy(out[0:32], u256Pad(base))
	copy(out[32:64], u256Pad(quote))
	return out, nil
}

func (p *orderbookPrecompile) erc20TransferFrom(token, from, to crypto.Address, amount *uint256.Int) error {
	if amount == nil || amount.IsZero() {
		return nil
	}
	if p.evm == nil {
		return fmt.Errorf("native swap: no evm for erc20")
	}
	data := make([]byte, 0, 4+32*3)
	data = append(data, selTransferFrom...)
	data = append(data, ethcommon.LeftPadBytes(from.Bytes(), 32)...)
	data = append(data, ethcommon.LeftPadBytes(to.Bytes(), 32)...)
	data = append(data, ethcommon.LeftPadBytes(amount.ToBig().Bytes(), 32)...)
	ret, _, err := p.evm.Call(
		toEthAddr(p.self),
		toEthAddr(token),
		data,
		ethvm.NewGasBudget(erc20CallGas, 0),
		uint256.NewInt(0),
	)
	if err != nil {
		return fmt.Errorf("native swap: transferFrom: %w", err)
	}
	if len(ret) > 0 && !erc20Success(ret) {
		return fmt.Errorf("native swap: transferFrom returned false")
	}
	return nil
}

func (p *orderbookPrecompile) erc20Transfer(token, to crypto.Address, amount *uint256.Int) error {
	if amount == nil || amount.IsZero() {
		return nil
	}
	if p.evm == nil {
		return fmt.Errorf("native swap: no evm for erc20")
	}
	data := make([]byte, 0, 4+32*2)
	data = append(data, selTransfer...)
	data = append(data, ethcommon.LeftPadBytes(to.Bytes(), 32)...)
	data = append(data, ethcommon.LeftPadBytes(amount.ToBig().Bytes(), 32)...)
	ret, _, err := p.evm.Call(
		toEthAddr(p.self),
		toEthAddr(token),
		data,
		ethvm.NewGasBudget(erc20CallGas, 0),
		uint256.NewInt(0),
	)
	if err != nil {
		return fmt.Errorf("native swap: transfer: %w", err)
	}
	if len(ret) > 0 && !erc20Success(ret) {
		return fmt.Errorf("native swap: transfer returned false")
	}
	return nil
}

func erc20Success(ret []byte) bool {
	if len(ret) == 0 {
		return true // some tokens return nothing
	}
	v := new(big.Int).SetBytes(ret)
	return v.Sign() != 0
}
