package consensus

import (
	"fmt"
	"math/big"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// BlockBuilder constructs a proposal block for a height.
// Tests and the local cluster supply a simple empty-block builder.
type BlockBuilder interface {
	// BuildProposal returns a block for height with ParentHash linked to parent.
	// stateRoot is the expected post-state commitment the proposer claims.
	BuildProposal(height uint64, parent *types.Header, proposer crypto.Address, stateRoot types.Hash) (*types.Block, error)
}

// ProposalValidator decides whether a proposed block is valid for prevote.
// Returning a non-nil error causes the engine to prevote nil.
type ProposalValidator interface {
	ValidateProposal(height uint64, parent *types.Header, block *types.Block) error
}

// EmptyBlockBuilder builds empty blocks (no txs) with the given gas limit and base fee.
type EmptyBlockBuilder struct {
	GasLimit uint64
	BaseFee  *big.Int
}

// BuildProposal implements BlockBuilder.
func (b *EmptyBlockBuilder) BuildProposal(height uint64, parent *types.Header, proposer crypto.Address, stateRoot types.Hash) (*types.Block, error) {
	if parent == nil {
		return nil, fmt.Errorf("consensus: missing parent header")
	}
	if height != parent.Number+1 {
		return nil, fmt.Errorf("consensus: height %d not parent+1 (%d)", height, parent.Number)
	}
	ts := parent.Timestamp + 1
	gl := b.GasLimit
	if gl == 0 {
		gl = parent.GasLimit
	}
	bf := b.BaseFee
	if bf == nil {
		if parent.BaseFee != nil {
			bf = new(big.Int).Set(parent.BaseFee)
		} else {
			bf = big.NewInt(1_000_000_000)
		}
	}
	h := &types.Header{
		ParentHash:  parent.Hash(),
		StateRoot:   stateRoot,
		TxRoot:      types.EmptyTxRoot,
		ReceiptRoot: types.EmptyReceiptRoot,
		Number:      height,
		Timestamp:   ts,
		GasLimit:    gl,
		GasUsed:     0,
		BaseFee:     bf,
		ExtraData:   nil,
		Proposer:    proposer,
	}
	return types.NewBlock(h, nil), nil
}

// DefaultValidator checks parent linkage, height, and optional expected state root.
type DefaultValidator struct {
	// ExpectedStateRoot, if non-zero, must match block.Header().StateRoot.
	// Used by tests to reject "invalid root" proposals.
	ExpectedStateRoot types.Hash
	// RequireExpected, when true, treats zero ExpectedStateRoot as "must be zero"
	// (rarely useful). When false, zero Expected means "do not check root".
	checkRoot bool
}

// NewRootValidator returns a ProposalValidator that rejects mismatched state roots.
func NewRootValidator(expected types.Hash) *DefaultValidator {
	return &DefaultValidator{ExpectedStateRoot: expected, checkRoot: true}
}

// NewBasicValidator returns a ProposalValidator that only checks header linkage.
func NewBasicValidator() *DefaultValidator {
	return &DefaultValidator{checkRoot: false}
}

// ValidateProposal implements ProposalValidator.
func (v *DefaultValidator) ValidateProposal(height uint64, parent *types.Header, block *types.Block) error {
	if block == nil {
		return fmt.Errorf("consensus: nil block")
	}
	if parent == nil {
		return fmt.Errorf("consensus: nil parent")
	}
	h := block.Header()
	if h.Number != height {
		return fmt.Errorf("consensus: block number %d != height %d", h.Number, height)
	}
	if h.ParentHash != parent.Hash() {
		return fmt.Errorf("consensus: parent hash mismatch")
	}
	if height != parent.Number+1 {
		return fmt.Errorf("consensus: height not contiguous")
	}
	if v.checkRoot && h.StateRoot != v.ExpectedStateRoot {
		return fmt.Errorf("consensus: invalid state root: got %s want %s", h.StateRoot.Hex(), v.ExpectedStateRoot.Hex())
	}
	return nil
}
