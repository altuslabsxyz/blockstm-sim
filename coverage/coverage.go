package coverage

import "sort"

// Entry describes a registered message type.
type Entry struct {
	Key       string `json:"key"`        // fixture TxSpec.Msg value, e.g. "bank-send"
	Module    string `json:"module"`     // e.g. "bank"
	MsgType   string `json:"msg_type"`   // e.g. "MsgSend"
	HandlerFn string `json:"handler_fn"` // e.g. "Send"
}

// EntryStat wraps an Entry with its execution count across a run.
type EntryStat struct {
	Entry
	Count int `json:"count"`
}

// Report is the per-run coverage summary produced by Tracker.Report.
type Report struct {
	Covered   []EntryStat // entries seen ≥1 time, sorted by Key
	Uncovered []Entry     // registered but never seen, sorted by Key
}

// Tracker accumulates which message types were executed across a run.
type Tracker struct {
	counts map[string]int // key → cumulative tx count
}

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{counts: make(map[string]int)}
}

// RecordBlock adds the msg keys from one block's transactions.
func (t *Tracker) RecordBlock(msgKeys []string) {
	for _, k := range msgKeys {
		t.counts[k]++
	}
}

// Report produces a coverage summary resolved against the global registry.
func (t *Tracker) Report() Report {
	reg := Registered()

	var covered []EntryStat
	var uncovered []Entry

	for k, e := range reg {
		if c := t.counts[k]; c > 0 {
			covered = append(covered, EntryStat{Entry: e, Count: c})
		} else {
			uncovered = append(uncovered, e)
		}
	}

	sort.Slice(covered, func(i, j int) bool { return covered[i].Key < covered[j].Key })
	sort.Slice(uncovered, func(i, j int) bool { return uncovered[i].Key < uncovered[j].Key })

	return Report{Covered: covered, Uncovered: uncovered}
}
