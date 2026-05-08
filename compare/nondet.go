package compare

// NonDetCall records a non-deterministic function call detected during block execution.
type NonDetCall struct {
	Category string // "time", "rand", "io"
	CallSite string // e.g. "time.Now", "rand.Intn"
	TxIndex  int    // -1 if attribution is unknown (parallel execution)
}

// NonDetProvider supplies non-deterministic calls recorded during oracle FinalizeBlock.
// Implemented by simharness.GlobalSink when built with -tags simharness.
type NonDetProvider interface {
	// NonDetCalls returns all calls recorded since the last Drain and resets the buffer.
	NonDetCalls() []NonDetCall
	// SetCurrentTx informs the sink which transaction is currently executing.
	// Called by BlockObserver.OnTxStart so calls can be attributed per-tx.
	SetCurrentTx(txIndex int)
}
