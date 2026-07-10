package types

import (
	"math/big"

	"github.com/holiman/uint256"
)

// Account is the flat account object stored in state_db (see docs/protocol/state.md).
// Storage slots live in storage_db, keyed by (address, slot).
type Account struct {
	Nonce    uint64
	Balance  *uint256.Int // wei; nil treated as zero
	CodeHash Hash         // EmptyCodeHash for EOAs
}

// NewAccount returns a zero-nonce EOA with zero balance.
func NewAccount() *Account {
	return &Account{
		Nonce:    0,
		Balance:  uint256.NewInt(0),
		CodeHash: EmptyCodeHash,
	}
}

// Copy returns a deep copy.
func (a *Account) Copy() *Account {
	if a == nil {
		return nil
	}
	out := &Account{
		Nonce:    a.Nonce,
		CodeHash: a.CodeHash,
	}
	if a.Balance != nil {
		out.Balance = new(uint256.Int).Set(a.Balance)
	} else {
		out.Balance = uint256.NewInt(0)
	}
	return out
}

// GetBalance returns balance or zero if nil.
func (a *Account) GetBalance() *uint256.Int {
	if a == nil || a.Balance == nil {
		return uint256.NewInt(0)
	}
	return a.Balance
}

// SetBalance sets balance from a big.Int (genesis JSON often uses decimal strings → big.Int).
func (a *Account) SetBalanceBig(v *big.Int) {
	if a.Balance == nil {
		a.Balance = uint256.NewInt(0)
	}
	if v == nil {
		a.Balance.Clear()
		return
	}
	a.Balance.SetFromBig(v)
}

// IsEOA reports whether the account has no code.
func (a *Account) IsEOA() bool {
	return a.CodeHash == EmptyCodeHash
}

// accountRLP is the canonical RLP shape for hashing/persistence.
type accountRLP struct {
	Nonce    uint64
	Balance  *big.Int
	CodeHash []byte
}

func (a *Account) toRLP() accountRLP {
	bal := a.GetBalance().ToBig()
	return accountRLP{
		Nonce:    a.Nonce,
		Balance:  bal,
		CodeHash: a.CodeHash.Bytes(),
	}
}
