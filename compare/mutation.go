package compare

// MutationTracker is implemented by keepers that hold state outside the KVStore.
// BlockObserver calls SnapshotOutOfKVStoreState before and after each transaction
// on the oracle (sequential) runner to detect field mutations that bypass MVMemory.
//
// Implementations do NOT need to import this package — Go's structural typing
// allows the interface to be satisfied without an explicit declaration.
type MutationTracker interface {
	// TrackerName returns a human-readable identifier (e.g. "simcanary.sharedMap")
	// used in finding reports.
	TrackerName() string

	// SnapshotOutOfKVStoreState returns a deterministic byte serialization of
	// all out-of-KVStore mutable state. nil means no mutable state is present.
	// The encoding must be deterministic (sorted keys, fixed-width values).
	SnapshotOutOfKVStoreState() []byte
}

// MutationRecord captures a before→after snapshot diff for one MutationTracker
// within a single transaction.
type MutationRecord struct {
	Tracker string
	Before  []byte
	After   []byte
}

// MutationProvider returns out-of-KVStore mutations detected for a given tx index.
type MutationProvider interface {
	TxMutations(txIndex int) []MutationRecord
}
