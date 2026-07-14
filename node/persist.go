package node

import (
	"fmt"
	"math/big"

	dewtypes "github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/db"
)

// persistBlockLocked writes state dirty + chain indexes + tip in one batch (when Batcher available).
// Caller must hold n.mu. On success, updates in-memory maps via commitBlockLocked.
func (n *Node) persistBlockLocked(block *dewtypes.Block, results []txExecResult) error {
	if block == nil {
		return fmt.Errorf("node: nil block")
	}
	hdr := block.Header()
	blockHash := block.Hash()

	// Build in-memory indexes first (needed for encode), then durable write, then tip pointer.
	// We apply commitBlockLocked only after successful Write so memory matches disk.

	lookups := make([]*TxLookup, 0, len(results))
	for i, res := range results {
		lookups = append(lookups, &TxLookup{
			BlockHash:   blockHash,
			BlockNumber: hdr.Number,
			Index:       uint(i),
			Tx:          res.ethTx,
			DewTx:       res.dewTx,
			From:        res.from,
			TxHash:      res.txHash,
		})
		if res.receipt != nil {
			res.receipt.BlockHash = blockHash
			res.receipt.TransactionIndex = uint(i)
		}
	}

	batcher, ok := n.db.(db.Batcher)
	if !ok {
		// Fallback: direct puts if Database is not a Batcher (Pebble always is).
		if err := n.statedb.FlushDirtyTo(n.db); err != nil {
			return err
		}
		if err := n.writeChainKeys(n.db, block, results, lookups); err != nil {
			return err
		}
		n.statedb.ClearDirty()
		n.commitBlockLocked(block, results)
		return nil
	}

	batch := batcher.NewBatch()
	if err := n.statedb.FlushDirtyTo(batch); err != nil {
		return err
	}
	if err := n.writeChainKeys(batch, block, results, lookups); err != nil {
		return err
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("node: persist batch: %w", err)
	}
	n.statedb.ClearDirty()
	n.commitBlockLocked(block, results)
	return nil
}

type kvWriter interface {
	Put(key, value []byte) error
}

func (n *Node) writeChainKeys(w kvWriter, block *dewtypes.Block, results []txExecResult, lookups []*TxLookup) error {
	hdr := block.Header()
	blockHash := block.Hash()

	hb, err := encodeHeader(hdr)
	if err != nil {
		return err
	}
	if err := w.Put(headerKey(blockHash), hb); err != nil {
		return err
	}
	bb, err := encodeBody(block.Transactions())
	if err != nil {
		return err
	}
	if err := w.Put(bodyKey(blockHash), bb); err != nil {
		return err
	}
	if err := w.Put(canonicalKey(hdr.Number), blockHash.Bytes()); err != nil {
		return err
	}

	for i, res := range results {
		if res.receipt != nil {
			rb, err := encodeReceipt(res.receipt)
			if err != nil {
				return err
			}
			if err := w.Put(receiptKey(res.txHash), rb); err != nil {
				return err
			}
			// Secondary log index for O(range) eth_getLogs (Track 4 residual).
			for j, lg := range res.receipt.Logs {
				if lg == nil {
					continue
				}
				lb, err := encodeLogIndex(lg, res.txHash, blockHash)
				if err != nil {
					return err
				}
				if err := w.Put(logIndexKey(hdr.Number, uint(i), uint(j)), lb); err != nil {
					return err
				}
			}
		}
		if i < len(lookups) {
			tb, err := encodeTxLookup(lookups[i])
			if err != nil {
				return err
			}
			if err := w.Put(txLookupKey(res.txHash), tb); err != nil {
				return err
			}
		}
	}

	tip, err := encodeTip(hdr.Number, blockHash)
	if err != nil {
		return err
	}
	if err := w.Put(metaTipKey, tip); err != nil {
		return err
	}
	// Keep log-index schema marker current whenever we seal (covers genesis→first block).
	return putLogIndexVersion(w, logIndexSchemaVersion)
}

// persistGenesisLocked writes meta + genesis block chain keys after genesis state commit.
// State is already on db via statedb.Commit. Caller holds n.mu (or construction, no concurrent access).
func (n *Node) persistGenesisLocked(block *dewtypes.Block) error {
	if block == nil {
		return fmt.Errorf("node: nil genesis block")
	}
	blockHash := block.Hash()
	hdr := block.Header()

	batcher, ok := n.db.(db.Batcher)
	var w kvWriter
	var batch db.Batch
	if ok {
		batch = batcher.NewBatch()
		w = batch
	} else {
		w = n.db
	}

	if err := putMeta(w, n.chainID, blockHash); err != nil {
		return err
	}
	hb, err := encodeHeader(hdr)
	if err != nil {
		return err
	}
	if err := w.Put(headerKey(blockHash), hb); err != nil {
		return err
	}
	bb, err := encodeBody(nil)
	if err != nil {
		return err
	}
	if err := w.Put(bodyKey(blockHash), bb); err != nil {
		return err
	}
	if err := w.Put(canonicalKey(0), blockHash.Bytes()); err != nil {
		return err
	}
	tip, err := encodeTip(0, blockHash)
	if err != nil {
		return err
	}
	if err := w.Put(metaTipKey, tip); err != nil {
		return err
	}
	if err := putLogIndexVersion(w, logIndexSchemaVersion); err != nil {
		return err
	}
	if batch != nil {
		if err := batch.Write(); err != nil {
			return err
		}
	}
	return nil
}

// ensure chainID compare helper
func chainIDEqual(a, b *big.Int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Cmp(b) == 0
}
