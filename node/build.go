package node

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	dewtypes "github.com/dewnetwork/dew/core/types"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/mempool"
)

// DefaultMaxTxsPerBlock is the phase-1 proposal cap (parity with dev auto-mine).
const DefaultMaxTxsPerBlock = 1

// BuildBlockFromPool builds a proposal block from pending mempool entries.
// Execution runs on a state snapshot; live chain head and pool are unchanged.
func (n *Node) BuildBlockFromPool(height uint64, parent *dewtypes.Header, proposer dewcrypto.Address, maxTxs int) (*dewtypes.Block, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if maxTxs <= 0 {
		maxTxs = DefaultMaxTxsPerBlock
	}
	if parent == nil {
		return nil, fmt.Errorf("node: nil parent")
	}
	if height != parent.Number+1 {
		return nil, fmt.Errorf("node: invalid height %d for parent %d", height, parent.Number)
	}

	newHeader := &dewtypes.Header{
		ParentHash:  parent.Hash(),
		Number:      height,
		Timestamp:   uint64(time.Now().Unix()),
		GasLimit:    parent.GasLimit,
		GasUsed:     0,
		BaseFee:     new(big.Int).Set(n.baseFee),
		ExtraData:   parent.ExtraData,
		Proposer:    proposer,
		StateRoot:   parent.StateRoot,
		TxRoot:      dewtypes.EmptyTxRoot,
		ReceiptRoot: dewtypes.EmptyReceiptRoot,
	}
	if newHeader.Timestamp <= parent.Timestamp {
		newHeader.Timestamp = parent.Timestamp + 1
	}

	txs := n.selectPendingTxsForBlockLocked(maxTxs)

	if len(txs) > 0 {
		root, totalGas, err := n.simulateBlockExecutionLocked(newHeader, txs)
		if err != nil {
			return nil, err
		}
		newHeader.StateRoot = root
		newHeader.GasUsed = totalGas
	} else if n.header.Number == parent.Number && n.header.Hash() == parent.Hash() {
		newHeader.StateRoot = n.header.StateRoot
	} else {
		root, err := n.statedb.IntermediateRoot()
		if err != nil {
			return nil, err
		}
		newHeader.StateRoot = root
	}

	return dewtypes.NewBlock(newHeader, txs), nil
}

// ValidateAndExecuteBlock re-executes block transactions against the current head
// state without committing to the live chain. It returns the computed post-state root.
func (n *Node) ValidateAndExecuteBlock(parent *dewtypes.Header, block *dewtypes.Block) (dewtypes.Hash, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.executeBlockLocked(parent, block)
}

func (n *Node) executeBlockLocked(parent *dewtypes.Header, block *dewtypes.Block) (dewtypes.Hash, error) {
	if parent == nil || block == nil {
		return dewtypes.Hash{}, fmt.Errorf("node: nil parent or block")
	}
	if n.header.Number != parent.Number || n.header.Hash() != parent.Hash() {
		return dewtypes.Hash{}, fmt.Errorf("node: head does not match parent")
	}

	hdr := block.Header()
	if len(block.Transactions()) == 0 {
		if hdr.GasUsed != 0 {
			return dewtypes.Hash{}, fmt.Errorf("node: gas used mismatch: got 0 want %d", hdr.GasUsed)
		}
		root, err := n.statedb.IntermediateRoot()
		if err != nil {
			return dewtypes.Hash{}, err
		}
		return root, nil
	}

	root, totalGas, err := n.simulateBlockExecutionLocked(hdr, block.Transactions())
	if err != nil {
		return dewtypes.Hash{}, err
	}
	if totalGas != hdr.GasUsed {
		return dewtypes.Hash{}, fmt.Errorf("node: gas used mismatch: got %d want %d", totalGas, hdr.GasUsed)
	}
	return root, nil
}

// simulateBlockExecutionLocked executes block txs on a throwaway state copy.
// Caller must hold n.mu.
func (n *Node) simulateBlockExecutionLocked(hdr *dewtypes.Header, txs []*dewtypes.Transaction) (dewtypes.Hash, uint64, error) {
	workDB := n.statedb.Copy()
	saved := n.statedb
	n.statedb = workDB
	defer func() { n.statedb = saved }()

	_, totalGas, err := n.executeBlockTxsLocked(hdr, txs)
	if err != nil {
		return dewtypes.Hash{}, 0, err
	}
	root, err := workDB.IntermediateRoot()
	if err != nil {
		return dewtypes.Hash{}, 0, err
	}
	return root, totalGas, nil
}

// selectPendingTxsForBlockLocked returns up to maxTxs executable EVM transactions,
// choosing highest price first among entries whose nonce matches current state.
// Caller must hold n.mu.
func (n *Node) selectPendingTxsForBlockLocked(maxTxs int) []*dewtypes.Transaction {
	pending := n.pool.Pending()
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Price.Cmp(pending[j].Price) > 0
	})

	var out []*dewtypes.Transaction
	for _, e := range pending {
		if len(out) >= maxTxs {
			break
		}
		if e.Kind != mempool.KindEVM {
			continue
		}
		if e.Nonce != n.statedb.GetNonce(e.From) {
			continue
		}
		ethTx := new(ethtypes.Transaction)
		if err := ethTx.UnmarshalBinary(e.Raw); err != nil {
			continue
		}
		dewTx, err := ethTxToDew(ethTx)
		if err != nil {
			continue
		}
		out = append(out, dewTx)
	}
	return out
}