package vm

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/crypto"
)

// ExecutionStats holds Dew-PE operational metrics (see dew_getExecutionStats).
type ExecutionStats struct {
	TxCount           int
	Workers           int
	Rollbacks         int
	SpeculativeOK     int
	DurationNS        int64
	ConflictRate      float64 // Rollbacks / TxCount
	ActiveWorkers     int
	TotalTxs          uint64
	TotalRollbacks    uint64
	PeakWorkers       int
	CurrentTPS        float64
	PeakTPS           float64
	StateDBReadLatNS  int64
	StateDBWriteLatNS int64
}

// ParallelExecutor runs a block of messages with optimistic Block-STM style concurrency.
// Final state is equivalent to sequential ApplyMessage in index order.
type ParallelExecutor struct {
	statedb *state.StateDB
	block   BlockContext
	workers int

	mu    sync.Mutex
	stats ExecutionStats
}

// NewParallelExecutor creates a Dew-PE executor.
// workers <= 0 defaults to GOMAXPROCS.
func NewParallelExecutor(statedb *state.StateDB, block BlockContext, workers int) *ParallelExecutor {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers < 1 {
		workers = 1
	}
	return &ParallelExecutor{
		statedb: statedb,
		block:   block,
		workers: workers,
	}
}

// Stats returns a snapshot of execution metrics.
func (p *ParallelExecutor) Stats() ExecutionStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}

// ApplySequential runs messages in index order (baseline / fallback).
func (p *ParallelExecutor) ApplySequential(msgs []Message) ([]*Result, error) {
	out := make([]*Result, len(msgs))
	exec := NewExecutor(p.statedb, p.block)
	for i, msg := range msgs {
		res, err := exec.ApplyMessage(msg)
		if err != nil {
			return out, fmt.Errorf("vm/pe: sequential tx %d: %w", i, err)
		}
		out[i] = res
	}
	return out, nil
}

type speculativeResult struct {
	res *Result
	as  *state.AccessSet
	db  *state.StateDB
	err error
}

// ApplyParallel optimistically executes messages concurrently, then validates
// read/write sets in index order. Conflicting txs are re-executed on the
// committed prefix (serial-equivalent semantics).
func (p *ParallelExecutor) ApplyParallel(msgs []Message) ([]*Result, error) {
	if len(msgs) == 0 {
		return nil, nil
	}
	if len(msgs) == 1 || p.workers == 1 {
		start := time.Now()
		res, err := p.ApplySequential(msgs)
		p.recordStats(len(msgs), 1, 0, len(msgs), time.Since(start))
		return res, err
	}

	start := time.Now()
	specs := make([]speculativeResult, len(msgs))
	parentSnapshot := p.statedb

	sem := make(chan struct{}, p.workers)
	var wg sync.WaitGroup
	var active int32

	for i := range msgs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			atomic.AddInt32(&active, 1)
			defer func() {
				atomic.AddInt32(&active, -1)
				<-sem
			}()

			fork := parentSnapshot.Copy()
			fork.StartAccessTracking()
			exec := NewExecutor(fork, p.block)
			res, err := exec.ApplyMessage(msgs[i])
			as := fork.TakeAccessSet()
			specs[i] = speculativeResult{res: res, as: as, db: fork, err: err}
		}(i)
	}
	wg.Wait()

	results := make([]*Result, len(msgs))
	earlierWrites := make(map[state.AccessKey]struct{})
	rollbacks := 0
	specOK := 0

	for i := range msgs {
		if specs[i].err != nil || specs[i].as == nil || specs[i].as.ConflictsWith(earlierWrites) {
			exec := NewExecutor(p.statedb, p.block)
			// Track writes during re-exec for accurate dependency chain.
			p.statedb.StartAccessTracking()
			res, err := exec.ApplyMessage(msgs[i])
			as := p.statedb.TakeAccessSet()
			if err != nil {
				return results, fmt.Errorf("vm/pe: re-exec tx %d: %w", i, err)
			}
			results[i] = res
			rollbacks++
			if as != nil {
				as.MergeWrites(earlierWrites)
			} else {
				recordMessageWrites(earlierWrites, msgs[i], p.block.Coinbase)
			}
			continue
		}

		// Accept speculative result: merge overlay into live state.
		p.statedb.ApplyOverlay(specs[i].db)
		// Match sequential ApplyMessage: Finalise purges empty accounts so
		// IntermediateRoot / cache occupancy stay serial-equivalent.
		p.statedb.Finalise(true)
		results[i] = specs[i].res
		specs[i].as.MergeWrites(earlierWrites)
		specOK++
	}

	p.recordStats(len(msgs), p.workers, rollbacks, specOK, time.Since(start))
	return results, nil
}

func recordMessageWrites(dst map[state.AccessKey]struct{}, msg Message, coinbase crypto.Address) {
	dst[state.AccountKey(msg.From)] = struct{}{}
	if msg.To != nil {
		dst[state.AccountKey(*msg.To)] = struct{}{}
	}
	dst[state.AccountKey(coinbase)] = struct{}{}
}

func (p *ParallelExecutor) recordStats(txCount, workers, rollbacks, specOK int, d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	rate := 0.0
	if txCount > 0 {
		rate = float64(rollbacks) / float64(txCount)
	}
	tps := 0.0
	if d > 0 {
		tps = float64(txCount) / d.Seconds()
	}
	p.stats.TxCount = txCount
	p.stats.Workers = workers
	p.stats.Rollbacks = rollbacks
	p.stats.SpeculativeOK = specOK
	p.stats.DurationNS = d.Nanoseconds()
	p.stats.ConflictRate = rate
	p.stats.ActiveWorkers = workers
	p.stats.TotalTxs += uint64(txCount)
	p.stats.TotalRollbacks += uint64(rollbacks)
	if workers > p.stats.PeakWorkers {
		p.stats.PeakWorkers = workers
	}
	p.stats.CurrentTPS = tps
	if tps > p.stats.PeakTPS {
		p.stats.PeakTPS = tps
	}
}
