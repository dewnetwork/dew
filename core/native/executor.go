// Package native implements Dew-native transaction execution (Phase B).
// Native txs bypass the EVM interpreter; AccessList is mandatory and fail-closed.
package native

import (
	"fmt"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

// Payload module tags (first byte of DewTx.Payload).
const (
	// PayloadEmpty: system token transfer only (Receiver + Amount).
	PayloadEmpty = 0x00
	// PayloadCredit: 0x01 || address(20) || amount(32) — secondary credit; must be in AccessList.
	PayloadCredit byte = 0x01
)

// Result is the outcome of applying a DewTx.
type Result struct {
	FeePaid uint64
	Failed  bool
	Err     error
	Touched []crypto.Address
}

// Executor applies DewTx against a StateDB.
type Executor struct {
	statedb *state.StateDB
	feeSink crypto.Address // receives flat fees (coinbase / proposer)
}

// NewExecutor builds a native executor. feeSink receives flat fees (typically block proposer).
func NewExecutor(statedb *state.StateDB, feeSink crypto.Address) *Executor {
	return &Executor{statedb: statedb, feeSink: feeSink}
}

// ApplyDewTx executes a verified DewTx.
//
// Fail-closed access control: every account read or written (except the protocol
// fee sink) must be tx.Sender, tx.Receiver, or listed in AccessList. Incomplete
// lists return a failed result (not a panic, not silent privilege escalation).
func (e *Executor) ApplyDewTx(tx *types.DewTx) (*Result, error) {
	if tx == nil {
		return nil, fmt.Errorf("native: nil tx")
	}
	if tx.Version != params.DewTxVersion {
		return nil, fmt.Errorf("native: unsupported version %d", tx.Version)
	}
	if tx.Amount == nil {
		tx.Amount = uint256.NewInt(0)
	}
	fee := tx.Fee
	if fee == 0 {
		fee = params.DefaultDewTxFeeWei
	}

	allowed := map[crypto.Address]struct{}{
		tx.Sender:   {},
		tx.Receiver: {},
	}
	for _, a := range tx.AccessList {
		allowed[a] = struct{}{}
	}
	// Protocol fee sink is always permitted (not a user privilege escalation).
	allowed[e.feeSink] = struct{}{}

	require := func(addr crypto.Address) error {
		if _, ok := allowed[addr]; !ok {
			return fmt.Errorf("native: access list incomplete: touched %s", addr.Hex())
		}
		return nil
	}

	// Nonce / balance checks (outer errors — reject from mempool)
	if e.statedb.GetNonce(tx.Sender) != tx.Nonce {
		return nil, fmt.Errorf("native: bad nonce: got tx %d state %d", tx.Nonce, e.statedb.GetNonce(tx.Sender))
	}

	// Parse optional module before mutating state
	var extraAddr *crypto.Address
	var extraAmt *uint256.Int
	if len(tx.Payload) > 0 {
		switch tx.Payload[0] {
		case PayloadCredit:
			if len(tx.Payload) != 1+20+32 {
				return failResult(fmt.Errorf("native: invalid credit payload length")), nil
			}
			var a crypto.Address
			copy(a[:], tx.Payload[1:21])
			extraAddr = &a
			extraAmt = new(uint256.Int).SetBytes(tx.Payload[21:53])
			if err := require(a); err != nil {
				return failResult(err), nil
			}
		default:
			return failResult(fmt.Errorf("native: unknown payload module 0x%02x", tx.Payload[0])), nil
		}
	}

	if err := require(tx.Sender); err != nil {
		return failResult(err), nil
	}
	if err := require(tx.Receiver); err != nil {
		return failResult(err), nil
	}

	// Total debit = Amount + extraAmt + fee
	need := new(uint256.Int).Set(tx.Amount)
	if extraAmt != nil {
		need = need.Add(need, extraAmt)
	}
	need = need.Add(need, uint256.NewInt(fee))
	if e.statedb.GetBalance(tx.Sender).Cmp(need) < 0 {
		return nil, fmt.Errorf("native: insufficient balance")
	}

	// Mutate
	e.statedb.SubBalance(tx.Sender, need)
	if !tx.Amount.IsZero() {
		e.statedb.AddBalancePrev(tx.Receiver, tx.Amount)
	}
	if extraAddr != nil && extraAmt != nil && !extraAmt.IsZero() {
		e.statedb.AddBalancePrev(*extraAddr, extraAmt)
	}
	if fee > 0 {
		e.statedb.AddBalancePrev(e.feeSink, uint256.NewInt(fee))
	}
	e.statedb.SetNonceJournaled(tx.Sender, tx.Nonce+1)
	e.statedb.Finalise(true)

	touched := []crypto.Address{tx.Sender, tx.Receiver, e.feeSink}
	if extraAddr != nil {
		touched = append(touched, *extraAddr)
	}
	return &Result{FeePaid: fee, Failed: false, Touched: touched}, nil
}

func failResult(err error) *Result {
	return &Result{Failed: true, Err: err}
}
