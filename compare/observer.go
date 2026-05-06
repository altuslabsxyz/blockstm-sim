//go:build sdk_hooks

package compare

import (
	"bytes"
	"encoding/hex"
	"sort"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/baseapp/lifecycle"
)

// BlockObserver implements lifecycle.LifecycleObserver to capture per-transaction
// KVStore write keys and out-of-KVStore field mutations during FinalizeBlock.
//
// Write set tracking: each txIndex owns its own map, so concurrent OnKVWrite
// calls from BlockSTM goroutines are safe without a mutex as long as each
// goroutine writes to a distinct txIndex.
//
// Mutation tracking: uses block-level capture via CaptureBeforeBlock /
// CaptureAfterBlock. The caller snapshots state before oracle FinalizeBlock and
// diffs after; the delta is attributed to TxIndex=0. This avoids dependence on
// OnTxStart/OnTxEnd, which are not guaranteed to fire in all SDK runners.
//
// Important: the SDK's executeTxsWithExecutor calls OnTxStart/OnTxEnd in a
// post-hoc loop AFTER txRunner.Run returns, so all transactions have already
// finished executing when those callbacks fire. OnTxStart(0) would overwrite
// snapshots[0] with the already-mutated state, breaking CaptureAfterBlock.
// blockSnapshots is a dedicated field that OnTxStart cannot touch, ensuring
// CaptureBeforeBlock's baseline survives the post-hoc callback loop.
type BlockObserver struct {
	lifecycle.NoopLifecycleObserver
	writeSets []map[string]struct{}

	trackers       []MutationTracker
	snapshots      [][][]byte         // snapshots[txIdx][trackerIdx] = pre-tx snapshot
	blockSnapshots [][]byte           // pre-block baseline for CaptureBeforeBlock/CaptureAfterBlock
	mutSets        [][]MutationRecord // mutSets[txIdx] = mutations detected in tx
	txSetters      []TxIndexSetter
}

var _ lifecycle.LifecycleObserver = (*BlockObserver)(nil)
var _ WriteSetProvider = (*BlockObserver)(nil)
var _ MutationProvider = (*BlockObserver)(nil)

func NewBlockObserver(txCount int, trackers ...MutationTracker) *BlockObserver {
	ws := make([]map[string]struct{}, txCount)
	for i := range ws {
		ws[i] = make(map[string]struct{})
	}
	snaps := make([][][]byte, txCount)
	for i := range snaps {
		snaps[i] = make([][]byte, len(trackers))
	}
	var setters []TxIndexSetter
	for _, tracker := range trackers {
		if setter, ok := tracker.(TxIndexSetter); ok {
			setters = append(setters, setter)
		}
	}
	blockSnaps := make([][]byte, len(trackers))
	return &BlockObserver{
		writeSets:      ws,
		trackers:       trackers,
		snapshots:      snaps,
		blockSnapshots: blockSnaps,
		mutSets:        make([][]MutationRecord, txCount),
		txSetters:      setters,
	}
}

func (o *BlockObserver) OnTxStart(txIndex int) {
	if txIndex < len(o.writeSets) {
		o.writeSets[txIndex] = make(map[string]struct{})
	}
	for _, setter := range o.txSetters {
		setter.SetCurrentTx(txIndex)
	}
	if txIndex < len(o.snapshots) {
		for i, t := range o.trackers {
			o.snapshots[txIndex][i] = t.SnapshotOutOfKVStoreState()
		}
	}
}

func (o *BlockObserver) OnKVWrite(storeKey string, key []byte, txIndex int) {
	if txIndex < len(o.writeSets) {
		composite := storeKey + "/" + hex.EncodeToString(key)
		o.writeSets[txIndex][composite] = struct{}{}
	}
}

func (o *BlockObserver) OnTxEnd(txIndex int, _ *abci.ExecTxResult) {
	if txIndex >= len(o.snapshots) || len(o.trackers) == 0 {
		return
	}
	for i, t := range o.trackers {
		after := t.SnapshotOutOfKVStoreState()
		before := o.snapshots[txIndex][i]
		if !bytes.Equal(before, after) {
			o.mutSets[txIndex] = append(o.mutSets[txIndex], MutationRecord{
				Tracker: t.TrackerName(),
				Before:  before,
				After:   after,
			})
		}
	}
}

// TxWriteSet returns the sorted list of composite write keys for the given tx.
func (o *BlockObserver) TxWriteSet(txIndex int) []string {
	if txIndex >= len(o.writeSets) {
		return nil
	}
	m := o.writeSets[txIndex]
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TxMutations returns the out-of-KVStore mutations detected for the given tx.
func (o *BlockObserver) TxMutations(txIndex int) []MutationRecord {
	if txIndex >= len(o.mutSets) {
		return nil
	}
	return o.mutSets[txIndex]
}

// AddBlockMutation appends a mutation record to the block-level slot (txIndex=0).
// This is the direct alternative to CaptureBeforeBlock/CaptureAfterBlock for
// callers that manage snapshot diffing outside the observer.
func (o *BlockObserver) AddBlockMutation(m MutationRecord) {
	if len(o.mutSets) > 0 {
		o.mutSets[0] = append(o.mutSets[0], m)
	}
}

// CaptureBeforeBlock snapshots all tracker states into blockSnapshots. Call
// this immediately before the oracle FinalizeBlock to establish a baseline.
func (o *BlockObserver) CaptureBeforeBlock() {
	if len(o.trackers) == 0 || len(o.blockSnapshots) == 0 {
		return
	}
	for i, t := range o.trackers {
		o.blockSnapshots[i] = t.SnapshotOutOfKVStoreState()
	}
}

// CaptureAfterBlock diffs current tracker states against the pre-block
// baseline in blockSnapshots and appends any changes to mutSets[0]. Call this
// immediately after oracle FinalizeBlock returns.
func (o *BlockObserver) CaptureAfterBlock() {
	if len(o.trackers) == 0 || len(o.mutSets) == 0 || len(o.blockSnapshots) == 0 {
		return
	}
	for i, t := range o.trackers {
		after := t.SnapshotOutOfKVStoreState()
		before := o.blockSnapshots[i]
		if !bytes.Equal(before, after) {
			o.mutSets[0] = append(o.mutSets[0], MutationRecord{
				Tracker: t.TrackerName(),
				Before:  before,
				After:   after,
			})
		}
	}
}
