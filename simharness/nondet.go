//go:build simharness

package simharness

import (
	"sync"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// GlobalSink is the package-level non-deterministic call sink.
// It is registered when built with -tags simharness and is wired into the
// comparison executor as a compare.NonDetProvider.
//
// Integration point for real shims: any wrapper around time.Now(), rand, or
// I/O packages should call GlobalSink.Record(...) to feed into the pipeline.
// Example:
//
//	func TimeNow() time.Time {
//	    GlobalSink.Record("time", "time.Now")
//	    return time.Now()
//	}
var GlobalSink = &nonDetSink{txIndex: -1}

// Provider returns GlobalSink as a compare.NonDetProvider.
// Imported by run/executor.go so it doesn't need a simharness build tag check.
func Provider() compare.NonDetProvider { return GlobalSink }

// RecordCall records a non-deterministic call attributed to the current transaction.
// Safe to call from any goroutine; attribution uses the tx index set by SetCurrentTx.
func RecordCall(category, callSite string) {
	GlobalSink.Record(category, callSite)
}

// nonDetSink is a thread-safe buffer of NonDetCalls.
type nonDetSink struct {
	mu      sync.Mutex
	txIndex int
	calls   []compare.NonDetCall
}

func (s *nonDetSink) Record(category, callSite string) {
	s.mu.Lock()
	s.calls = append(s.calls, compare.NonDetCall{
		Category: category,
		CallSite: callSite,
		TxIndex:  s.txIndex,
	})
	s.mu.Unlock()
}

// SetCurrentTx sets the transaction index attributed to subsequent Record calls.
// Called by the executor's OnTxStart hook.
func (s *nonDetSink) SetCurrentTx(txIndex int) {
	s.mu.Lock()
	s.txIndex = txIndex
	s.mu.Unlock()
}

// NonDetCalls returns all recorded calls and resets the buffer.
func (s *nonDetSink) NonDetCalls() []compare.NonDetCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.calls
	s.calls = nil
	s.txIndex = -1
	return out
}
