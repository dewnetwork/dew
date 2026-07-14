package node

import (
	"fmt"

	dewtypes "github.com/dewnetwork/dew/core/types"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

// ensureLogIndex builds secondary log-index keys when missing (upgrade path for
// chaindata written before the index existed). Safe to call on every Open.
func (n *Node) ensureLogIndex() error {
	ver, ok, err := readLogIndexVersion(n.db)
	if err != nil {
		return err
	}
	if ok && ver == logIndexSchemaVersion {
		return nil
	}
	return n.rebuildLogIndex()
}

// rebuildLogIndex scans durable receipts and writes L|block|tx|log keys.
func (n *Node) rebuildLogIndex() error {
	it, ok := n.db.(db.IteratePrefix)
	if !ok {
		return fmt.Errorf("node: database does not support prefix iteration for log index rebuild")
	}

	type entry struct {
		key, value []byte
	}
	var entries []entry

	err := it.IteratePrefix([]byte{prefixReceipt}, func(key, value []byte) error {
		if len(key) != 1+32 {
			return nil
		}
		txHash := dewtypes.BytesToHash(key[1:])
		rcpt, err := decodeReceipt(value)
		if err != nil {
			return nil
		}
		for j, lg := range rcpt.Logs {
			if lg == nil {
				continue
			}
			lb, err := encodeLogIndex(lg, txHash, rcpt.BlockHash)
			if err != nil {
				return err
			}
			k := logIndexKey(rcpt.BlockNumber, rcpt.TransactionIndex, uint(j))
			entries = append(entries, entry{
				key:   append([]byte(nil), k...),
				value: lb,
			})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("node: log index rebuild scan: %w", err)
	}

	batcher, ok := n.db.(db.Batcher)
	if !ok {
		for _, e := range entries {
			if err := n.db.Put(e.key, e.value); err != nil {
				return err
			}
		}
		return putLogIndexVersion(n.db, logIndexSchemaVersion)
	}

	batch := batcher.NewBatch()
	for _, e := range entries {
		if err := batch.Put(e.key, e.value); err != nil {
			return err
		}
	}
	if err := putLogIndexVersion(batch, logIndexSchemaVersion); err != nil {
		return err
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("node: log index rebuild batch: %w", err)
	}
	return nil
}

// filterLogsFromIndexLocked walks L keys for [fromBlock, toBlock] only (O(range)).
// Caller holds n.mu.
func (n *Node) filterLogsFromIndexLocked(fromBlock, toBlock uint64, add func(*IndexedLog)) {
	it, ok := n.db.(db.IteratePrefix)
	if !ok {
		return
	}
	if toBlock < fromBlock {
		return
	}
	for b := fromBlock; b <= toBlock; b++ {
		prefix := logIndexBlockPrefix(b)
		_ = it.IteratePrefix(prefix, func(key, value []byte) error {
			blockNum, txIndex, logIdx, ok := parseLogIndexKey(key)
			if !ok {
				return nil
			}
			lg, txHash, blockHash, err := decodeLogIndex(value)
			if err != nil || lg == nil {
				return nil
			}
			add(&IndexedLog{
				Log:         lg,
				BlockNumber: blockNum,
				BlockHash:   blockHash,
				TxHash:      txHash,
				TxIndex:     txIndex,
				Index:       logIdx,
			})
			return nil
		})
	}
}

// countLogIndexKeysInRange counts durable log-index entries in [from, to] (tests / ops).
func (n *Node) countLogIndexKeysInRange(fromBlock, toBlock uint64) int {
	it, ok := n.db.(db.IteratePrefix)
	if !ok {
		return 0
	}
	var nKeys int
	for b := fromBlock; b <= toBlock; b++ {
		_ = it.IteratePrefix(logIndexBlockPrefix(b), func(key, value []byte) error {
			if _, _, _, ok := parseLogIndexKey(key); ok {
				nKeys++
			}
			return nil
		})
	}
	return nKeys
}

// countReceiptKeys returns the number of durable receipt keys (tests).
func (n *Node) countReceiptKeys() int {
	it, ok := n.db.(db.IteratePrefix)
	if !ok {
		return 0
	}
	var nKeys int
	_ = it.IteratePrefix([]byte{prefixReceipt}, func(key, value []byte) error {
		if len(key) == 1+32 {
			nKeys++
		}
		return nil
	})
	return nKeys
}

// hasLogIndexVersion reports whether the secondary log-index meta marker is current.
func (n *Node) hasLogIndexVersion() bool {
	ver, ok, err := readLogIndexVersion(n.db)
	return err == nil && ok && ver == logIndexSchemaVersion
}

// logMatchesFilter is shared by FilterLogs address/topic checks.
func logMatchesFilter(il *IndexedLog, fromBlock, toBlock uint64, addrSet map[dewcrypto.Address]struct{}, topics [][]dewtypes.Hash) bool {
	if il == nil || il.Log == nil {
		return false
	}
	if il.BlockNumber < fromBlock || il.BlockNumber > toBlock {
		return false
	}
	if len(addrSet) > 0 {
		if _, ok := addrSet[il.Log.Address]; !ok {
			return false
		}
	}
	return matchTopics(il.Log.Topics, topics)
}
