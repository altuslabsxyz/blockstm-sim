package compare

import (
	"encoding/hex"
	"sort"
	"sync"
)

// ConflictRecord is one BlockSTM read-set validation conflict: transaction
// TxIndex's recorded read of Key in store Store no longer reproduced, forcing
// a re-execution. It mirrors the shape of the SDK-side conflict record so that
// adapters can convert between the two without this package importing any
// fork-specific packages.
type ConflictRecord struct {
	Store   string
	TxIndex int
	Key     []byte
	Reason  string // short reason label supplied by the adapter
}

// ExecutionStats summarizes one BlockSTM run: the scheduler's final counters
// when the block completed. ExecutedTxns counts every execution including
// re-executions, so ExecutedTxns/BlockSize is the block's execution ratio
// (1.0 = no re-execution).
type ExecutionStats struct {
	BlockSize     int
	ExecutedTxns  int64
	ValidatedTxns int64
}

// Ratio returns ExecutedTxns/BlockSize, or 0 when BlockSize is 0.
func (s ExecutionStats) Ratio() float64 {
	if s.BlockSize == 0 {
		return 0
	}
	return float64(s.ExecutedTxns) / float64(s.BlockSize)
}

// ConflictSink buffers conflict records and execution stats reported during
// one probe FinalizeBlock. Safe for concurrent use: BlockSTM invokes the
// conflict observer from multiple executor goroutines.
type ConflictSink struct {
	mu      sync.Mutex
	records []ConflictRecord
	stats   *ExecutionStats
}

// NewConflictSink returns an empty ConflictSink.
func NewConflictSink() *ConflictSink { return &ConflictSink{} }

// RecordConflict appends one conflict record.
func (s *ConflictSink) RecordConflict(r ConflictRecord) {
	s.mu.Lock()
	s.records = append(s.records, r)
	s.mu.Unlock()
}

// RecordStats stores the block's final scheduler counters.
func (s *ConflictSink) RecordStats(st ExecutionStats) {
	s.mu.Lock()
	s.stats = &st
	s.mu.Unlock()
}

// Drain returns all buffered records and stats and resets the sink.
// stats is nil when no execution stats were reported.
func (s *ConflictSink) Drain() (records []ConflictRecord, stats *ExecutionStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, stats = s.records, s.stats
	s.records, s.stats = nil, nil
	return records, stats
}

// HotKeyStat aggregates the validation conflicts on one store/key within a
// single block. A key that many distinct transactions re-executed on is a
// contention hotspot (e.g. an account rewritten by every tx).
type HotKeyStat struct {
	Store     string
	Key       string // hex-encoded raw key
	Conflicts int    // total conflict records on this key
	Txs       []int  // sorted distinct tx indices that re-executed because of it
}

// AggregateHotKeys groups conflict records by (store, key) and returns one
// HotKeyStat per group, sorted by Conflicts descending, then Store, then Key.
func AggregateHotKeys(records []ConflictRecord) []HotKeyStat {
	if len(records) == 0 {
		return nil
	}
	type groupKey struct{ store, key string }
	type agg struct {
		conflicts int
		txs       map[int]struct{}
	}
	groups := make(map[groupKey]*agg)
	for _, r := range records {
		k := groupKey{store: r.Store, key: hex.EncodeToString(r.Key)}
		g := groups[k]
		if g == nil {
			g = &agg{txs: make(map[int]struct{})}
			groups[k] = g
		}
		g.conflicts++
		g.txs[r.TxIndex] = struct{}{}
	}

	out := make([]HotKeyStat, 0, len(groups))
	for k, g := range groups {
		txs := make([]int, 0, len(g.txs))
		for tx := range g.txs {
			txs = append(txs, tx)
		}
		sort.Ints(txs)
		out = append(out, HotKeyStat{Store: k.store, Key: k.key, Conflicts: g.conflicts, Txs: txs})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Conflicts != out[j].Conflicts {
			return out[i].Conflicts > out[j].Conflicts
		}
		if out[i].Store != out[j].Store {
			return out[i].Store < out[j].Store
		}
		return out[i].Key < out[j].Key
	})
	return out
}
