package node

import (
	"fmt"
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/native"
	"github.com/dewnetwork/dew/core/vm"
	dewtypes "github.com/dewnetwork/dew/core/types"
	dewcrypto "github.com/dewnetwork/dew/crypto"
)

// txExecResult captures execution output for one included transaction.
type txExecResult struct {
	txHash  dewtypes.Hash
	ethTx   *ethtypes.Transaction
	dewTx   *dewtypes.DewTx
	from    dewcrypto.Address
	receipt *dewtypes.Receipt
	logs    []*dewtypes.Log
}

// ImportCommittedBlock applies a BFT-committed or synced block without re-entering BFT.
// Idempotent when the same height and hash are already stored.
func (n *Node) ImportCommittedBlock(block *dewtypes.Block) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.importCommittedBlockLocked(block)
}

func (n *Node) importCommittedBlockLocked(block *dewtypes.Block) error {
	if block == nil {
		return fmt.Errorf("node: nil block")
	}
	h := block.Header()
	if existing, ok := n.blockNum[h.Number]; ok {
		if existing == block.Hash() {
			return nil
		}
		return fmt.Errorf("node: conflict at height %d", h.Number)
	}
	if h.Number != n.header.Number+1 {
		return fmt.Errorf("node: block %d not parent+1 of head %d", h.Number, n.header.Number)
	}
	if h.ParentHash != n.header.Hash() {
		return fmt.Errorf("node: parent hash mismatch")
	}

	snap := n.statedb.Snapshot()
	results, totalGas, err := n.executeBlockTxsLocked(h, block.Transactions())
	if err != nil {
		n.statedb.RevertToSnapshot(snap)
		return err
	}
	if totalGas != h.GasUsed {
		n.statedb.RevertToSnapshot(snap)
		return fmt.Errorf("node: gas used mismatch: got %d want %d", totalGas, h.GasUsed)
	}
	root, err := n.statedb.IntermediateRoot()
	if err != nil {
		n.statedb.RevertToSnapshot(snap)
		return err
	}
	if root != h.StateRoot {
		n.statedb.RevertToSnapshot(snap)
		return fmt.Errorf("node: state root mismatch: got %s want %s", root.Hex(), h.StateRoot.Hex())
	}
	if err := n.persistBlockLocked(block, results); err != nil {
		n.statedb.RevertToSnapshot(snap)
		return err
	}
	return nil
}

func (n *Node) executeBlockTxsLocked(hdr *dewtypes.Header, txs []*dewtypes.Transaction) ([]txExecResult, uint64, error) {
	var results []txExecResult
	var cumulative uint64
	for i, tx := range txs {
		if tx == nil {
			continue
		}
		res, used, err := n.executeEVMTxLocked(hdr, tx, uint(i), cumulative)
		if err != nil {
			return nil, 0, err
		}
		cumulative = used
		results = append(results, res)
	}
	return results, cumulative, nil
}

func (n *Node) executeEVMTxLocked(hdr *dewtypes.Header, dewTx *dewtypes.Transaction, txIndex uint, cumulativeGasStart uint64) (txExecResult, uint64, error) {
	ethTx, err := dewTxToEth(dewTx)
	if err != nil {
		return txExecResult{}, cumulativeGasStart, err
	}
	signer := ethtypes.LatestSignerForChainID(n.chainID)
	fromEth, err := ethtypes.Sender(signer, ethTx)
	if err != nil {
		return txExecResult{}, cumulativeGasStart, fmt.Errorf("invalid signature: %w", err)
	}
	from := ethToDewAddr(fromEth)

	if ethTx.ChainId() != nil && ethTx.ChainId().Sign() != 0 && ethTx.ChainId().Cmp(n.chainID) != 0 {
		return txExecResult{}, cumulativeGasStart, fmt.Errorf("wrong chain id: got %s want %s", ethTx.ChainId(), n.chainID)
	}
	nonce := n.statedb.GetNonce(from)
	if ethTx.Nonce() != nonce {
		return txExecResult{}, cumulativeGasStart, fmt.Errorf("nonce too low/high: got %d want %d", ethTx.Nonce(), nonce)
	}

	baseFee := hdr.BaseFee
	if baseFee == nil {
		baseFee = n.baseFee
	}
	gasPrice := effectiveGasPrice(ethTx, baseFee)
	if gasPrice == nil {
		return txExecResult{}, cumulativeGasStart, fmt.Errorf("gas price too low for base fee")
	}

	msg := vm.Message{
		From:     from,
		Value:    uint256.MustFromBig(ethTx.Value()),
		GasLimit: ethTx.Gas(),
		GasPrice: gasPrice,
		Data:     ethTx.Data(),
	}
	if to := ethTx.To(); to != nil {
		a := ethToDewAddr(*to)
		msg.To = &a
	}

	exec := vm.NewExecutor(n.statedb, vm.BlockContext{
		Number:   hdr.Number,
		Time:     hdr.Timestamp,
		GasLimit: hdr.GasLimit,
		BaseFee:  baseFee,
		Coinbase: hdr.Proposer,
		ChainID:  n.chainID,
	})
	n.configureExecutor(exec)

	result, err := exec.ApplyMessage(msg)
	if err != nil {
		return txExecResult{}, cumulativeGasStart, err
	}

	txHash := dewtypes.BytesToHash(ethTx.Hash().Bytes())
	cumulative := cumulativeGasStart + result.UsedGas
	status := uint64(1)
	if result.Failed {
		status = 0
	}
	var contractAddr *dewcrypto.Address
	if result.ContractAddress != nil {
		contractAddr = result.ContractAddress
	}
	receipt := &dewtypes.Receipt{
		Type:              ethTx.Type(),
		Status:            status,
		CumulativeGasUsed: cumulative,
		GasUsed:           result.UsedGas,
		EffectiveGasPrice: gasPrice,
		Logs:              result.Logs,
		ContractAddress:   contractAddr,
		TxHash:            txHash,
		BlockNumber:       hdr.Number,
		TransactionIndex:  txIndex,
	}

	return txExecResult{
		txHash:  txHash,
		ethTx:   ethTx,
		from:    from,
		receipt: receipt,
		logs:    result.Logs,
	}, cumulative, nil
}

func (n *Node) executeDewTxLocked(hdr *dewtypes.Header, tx *dewtypes.DewTx, txIndex uint) (txExecResult, error) {
	if tx.ChainID == nil || tx.ChainID.Cmp(n.chainID) != 0 {
		return txExecResult{}, fmt.Errorf("wrong chain id: got %v want %s", tx.ChainID, n.chainID)
	}
	if _, err := tx.RecoverSender(); err != nil {
		return txExecResult{}, fmt.Errorf("invalid signature: %w", err)
	}

	feeSink := hdr.Proposer
	exec := native.NewExecutor(n.statedb, feeSink)
	if n.enableStaking {
		exec.EnableStaking(true, n.stakingConfigFromGenesis())
	}
	result, err := exec.ApplyDewTx(tx)
	if err != nil {
		return txExecResult{}, err
	}
	if result.Failed {
		return txExecResult{}, result.Err
	}

	txHash := tx.Hash()
	receipt := &dewtypes.Receipt{
		Type:              dewtypes.DewTxType,
		Status:            1,
		CumulativeGasUsed: 0,
		GasUsed:           0,
		EffectiveGasPrice: big.NewInt(0),
		Logs:              nil,
		TxHash:            txHash,
		BlockNumber:       hdr.Number,
		TransactionIndex:  txIndex,
	}
	return txExecResult{
		txHash:  txHash,
		dewTx:   tx,
		from:    tx.Sender,
		receipt: receipt,
		logs:    nil,
	}, nil
}

func (n *Node) commitBlockLocked(block *dewtypes.Block, results []txExecResult) {
	hdr := block.Header()
	blockHash := block.Hash()

	n.blocks[blockHash] = block
	n.blockNum[hdr.Number] = blockHash
	n.header = hdr.Copy()
	if hdr.BaseFee != nil {
		n.baseFee = new(big.Int).Set(hdr.BaseFee)
	}

	var sealLogs []*IndexedLog
	for i, res := range results {
		res.receipt.BlockHash = blockHash
		res.receipt.TransactionIndex = uint(i)
		n.txIndex[res.txHash] = &TxLookup{
			BlockHash:   blockHash,
			BlockNumber: hdr.Number,
			Index:       uint(i),
			Tx:          res.ethTx,
			DewTx:       res.dewTx,
			From:        res.from,
			TxHash:      res.txHash,
		}
		n.receipts[res.txHash] = res.receipt
		n.pool.Remove(res.txHash)
		for j, lg := range res.logs {
			il := &IndexedLog{
				Log:         lg,
				BlockNumber: hdr.Number,
				BlockHash:   blockHash,
				TxHash:      res.txHash,
				TxIndex:     uint(i),
				Index:       uint(j),
			}
			n.allLogs = append(n.allLogs, il)
			sealLogs = append(sealLogs, il)
		}
	}

	// Fan-out for eth_subscribe (newHeads / logs). Non-blocking; safe under n.mu.
	n.emitChainEventLocked(ChainEvent{
		Header: hdr.Copy(),
		Hash:   blockHash,
		Logs:   sealLogs,
	})
}

func ethTxToDew(tx *ethtypes.Transaction) (*dewtypes.Transaction, error) {
	raw, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	out := new(dewtypes.Transaction)
	if err := out.UnmarshalBinary(raw); err != nil {
		return nil, err
	}
	return out, nil
}

func dewTxToEth(tx *dewtypes.Transaction) (*ethtypes.Transaction, error) {
	raw, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	out := new(ethtypes.Transaction)
	if err := out.UnmarshalBinary(raw); err != nil {
		return nil, err
	}
	return out, nil
}

