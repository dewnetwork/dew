package node

import (
	"fmt"
	"math/big"
	"sort"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"

	dewtypes "github.com/dewnetwork/dew/core/types"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/mempool"
)

// DefaultMaxTxsPerBlock is the default proposal / auto-mine pack cap (C1 multi-tx).
const DefaultMaxTxsPerBlock = 64

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

	txs := n.selectPendingTxsForBlockLocked(maxTxs, parent.GasLimit)

	// Drop txs that fail simulation so one bad pending entry cannot halt proposals.
	if len(txs) > 0 {
		root, totalGas, err := n.simulateBlockExecutionLocked(newHeader, txs)
		if err != nil {
			txs = nil
		} else {
			newHeader.StateRoot = root
			newHeader.GasUsed = totalGas
			newHeader.TxRoot = dewtypes.TxRoot(txs)
		}
	}
	if len(txs) == 0 {
		if n.header.Number == parent.Number && n.header.Hash() == parent.Hash() {
			newHeader.StateRoot = n.header.StateRoot
		} else {
			root, err := n.statedb.IntermediateRoot()
			if err != nil {
				return nil, err
			}
			newHeader.StateRoot = root
		}
		newHeader.TxRoot = dewtypes.EmptyTxRoot
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

// selectPendingTxsForBlockLocked returns up to maxTxs executable EVM transactions.
// Selection is a fee auction over continuous per-sender nonce chains: among each
// sender's next executable nonce, pick the highest Price, advance that sender,
// repeat until maxTxs or gasLimit. Nonce gaps leave later txs pending.
// Caller must hold n.mu.
func (n *Node) selectPendingTxsForBlockLocked(maxTxs int, gasLimit uint64) []*dewtypes.Transaction {
	if maxTxs <= 0 {
		return nil
	}
	bySender := groupPendingBySender(n.pool.Pending(), mempool.KindEVM)

	nextNonce := make(map[dewcrypto.Address]uint64, len(bySender))
	for from := range bySender {
		nextNonce[from] = n.statedb.GetNonce(from)
	}

	var (
		out      []*dewtypes.Transaction
		totalGas uint64
	)
	for len(out) < maxTxs {
		best, bestFrom, ok := pickBestReady(bySender, nextNonce)
		if !ok {
			break
		}

		ethTx := new(ethtypes.Transaction)
		if err := ethTx.UnmarshalBinary(best.Raw); err != nil {
			// Drop unreadable entry from further consideration.
			bySender[bestFrom] = filterEntry(bySender[bestFrom], best.Hash)
			continue
		}
		if gasLimit > 0 && totalGas+ethTx.Gas() > gasLimit {
			// Cannot fit; stop considering this sender for this block.
			delete(bySender, bestFrom)
			continue
		}
		dewTx, err := ethTxToDew(ethTx)
		if err != nil {
			bySender[bestFrom] = filterEntry(bySender[bestFrom], best.Hash)
			continue
		}
		out = append(out, dewTx)
		totalGas += ethTx.Gas()
		nextNonce[bestFrom]++
	}
	return out
}

// selectPendingDewTxsForBlockLocked returns up to maxTxs executable DewTx entries
// with the same fee-auction / continuous-nonce selection as the EVM path.
// Flat-fee native txs do not consume EVM gas. Caller must hold n.mu.
func (n *Node) selectPendingDewTxsForBlockLocked(maxTxs int) []*dewtypes.DewTx {
	if maxTxs <= 0 {
		return nil
	}
	bySender := groupPendingBySender(n.pool.Pending(), mempool.KindDew)

	nextNonce := make(map[dewcrypto.Address]uint64, len(bySender))
	for from := range bySender {
		nextNonce[from] = n.statedb.GetNonce(from)
	}

	var out []*dewtypes.DewTx
	for len(out) < maxTxs {
		best, bestFrom, ok := pickBestReady(bySender, nextNonce)
		if !ok {
			break
		}
		dtx := new(dewtypes.DewTx)
		if err := dtx.UnmarshalBinary(best.Raw); err != nil {
			bySender[bestFrom] = filterEntry(bySender[bestFrom], best.Hash)
			continue
		}
		out = append(out, dtx)
		nextNonce[bestFrom]++
	}
	return out
}

// groupPendingBySender buckets pool entries of kind, each list sorted by nonce.
func groupPendingBySender(pending []*mempool.Entry, kind mempool.Kind) map[dewcrypto.Address][]*mempool.Entry {
	bySender := make(map[dewcrypto.Address][]*mempool.Entry)
	for _, e := range pending {
		if e == nil || e.Kind != kind {
			continue
		}
		bySender[e.From] = append(bySender[e.From], e)
	}
	for _, list := range bySender {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Nonce < list[j].Nonce
		})
	}
	return bySender
}

// pickBestReady finds the highest-Price entry whose nonce equals nextNonce[from].
func pickBestReady(
	bySender map[dewcrypto.Address][]*mempool.Entry,
	nextNonce map[dewcrypto.Address]uint64,
) (best *mempool.Entry, bestFrom dewcrypto.Address, ok bool) {
	for from, list := range bySender {
		want := nextNonce[from]
		var found *mempool.Entry
		for _, e := range list {
			if e.Nonce < want {
				continue
			}
			if e.Nonce > want {
				break // gap — nothing ready from this sender
			}
			found = e
			break
		}
		if found == nil {
			continue
		}
		if best == nil || found.Price.Cmp(best.Price) > 0 {
			best = found
			bestFrom = from
		}
	}
	if best == nil {
		return nil, dewcrypto.Address{}, false
	}
	return best, bestFrom, true
}

// dewHashListRoot commits ordered DewTx inclusion when the block body is empty.
// Same scheme as types.TxRoot: Keccak256(RLP([hash0, hash1, ...])).
func dewHashListRoot(results []txExecResult) dewtypes.Hash {
	if len(results) == 0 {
		return dewtypes.EmptyTxRoot
	}
	hashes := make([][]byte, len(results))
	for i, res := range results {
		hashes[i] = res.txHash.Bytes()
	}
	enc, err := rlp.EncodeToBytes(hashes)
	if err != nil {
		panic("node: dew hash list rlp: " + err.Error())
	}
	return dewtypes.Keccak256Hash(enc)
}

func filterEntry(list []*mempool.Entry, hash dewtypes.Hash) []*mempool.Entry {
	out := list[:0]
	for _, e := range list {
		if e != nil && e.Hash != hash {
			out = append(out, e)
		}
	}
	return out
}