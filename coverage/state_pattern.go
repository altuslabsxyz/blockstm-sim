package coverage

import (
	"sort"
	"strings"
)

// StatePatternTracker counts distinct KV write-set fingerprints per message type (F6 coverage).
type StatePatternTracker struct {
	patterns map[string]map[string]struct{}
}

// NewStatePatternTracker returns an empty tracker.
func NewStatePatternTracker() *StatePatternTracker {
	return &StatePatternTracker{patterns: make(map[string]map[string]struct{})}
}

// RecordBlock records patterns for one block. writeSets[i] is the sorted oracle write keys
// for msgKeys[i]; a nil/empty writeSets slice is skipped (no write-set data available).
func (t *StatePatternTracker) RecordBlock(msgKeys []string, writeSets [][]string) {
	if len(writeSets) == 0 {
		return
	}
	for i, key := range msgKeys {
		var fp string
		if i < len(writeSets) {
			fp = strings.Join(writeSets[i], "\x00")
		}
		if t.patterns[key] == nil {
			t.patterns[key] = make(map[string]struct{})
		}
		t.patterns[key][fp] = struct{}{}
	}
}

// StatePatternEntry holds the distinct write-set pattern count for one message type.
type StatePatternEntry struct {
	Key           string `json:"key"`
	DistinctCount int    `json:"distinct_count"`
}

// StatePatternReport is the per-run state-pattern coverage summary.
type StatePatternReport []StatePatternEntry

// DistinctCount returns the distinct pattern count for key, or 0 if not observed.
func (r StatePatternReport) DistinctCount(key string) int {
	for _, e := range r {
		if e.Key == key {
			return e.DistinctCount
		}
	}
	return 0
}

// Report produces the state pattern summary sorted by message key.
func (t *StatePatternTracker) Report() StatePatternReport {
	entries := make(StatePatternReport, 0, len(t.patterns))
	for key, fps := range t.patterns {
		entries = append(entries, StatePatternEntry{Key: key, DistinctCount: len(fps)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	return entries
}
