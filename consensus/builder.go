package consensus

import (
	"fmt"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// BlockExecutor builds and validates proposal blocks against chain state.
type BlockExecutor interface {
	BuildBlockFromPool(height uint64, parent *types.Header, proposer crypto.Address, maxTxs int) (*types.Block, error)
	ValidateAndExecuteBlock(parent *types.Header, block *types.Block) (types.Hash, error)
}

// MempoolBlockBuilder constructs proposals by draining the mempool via BlockExecutor.
type MempoolBlockBuilder struct {
	Exec   BlockExecutor
	MaxTxs int
}

// BuildProposal implements BlockBuilder.
func (b *MempoolBlockBuilder) BuildProposal(height uint64, parent *types.Header, proposer crypto.Address, _ types.Hash) (*types.Block, error) {
	if b == nil || b.Exec == nil {
		return nil, fmt.Errorf("consensus: nil mempool block builder")
	}
	maxTxs := b.MaxTxs
	if maxTxs <= 0 {
		maxTxs = 1
	}
	return b.Exec.BuildBlockFromPool(height, parent, proposer, maxTxs)
}

// ExecutionValidator re-executes proposals and rejects mismatched state roots.
type ExecutionValidator struct {
	Exec BlockExecutor
}

// ValidateProposal implements ProposalValidator.
func (v *ExecutionValidator) ValidateProposal(_ uint64, parent *types.Header, block *types.Block) error {
	if v == nil || v.Exec == nil {
		return fmt.Errorf("consensus: nil execution validator")
	}
	if block == nil {
		return fmt.Errorf("consensus: nil block")
	}
	root, err := v.Exec.ValidateAndExecuteBlock(parent, block)
	if err != nil {
		return err
	}
	if root != block.Header().StateRoot {
		return fmt.Errorf("consensus: invalid state root: got %s want %s", root.Hex(), block.Header().StateRoot.Hex())
	}
	return nil
}