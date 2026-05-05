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
type BlockObserver struct {
	lifecycle.NoopLifecycleObserver
	writeSets []map[string]struct{}

	trackers  []MutationTracker
	snapshots [][][]byte         // snapshots[txIdx][trackerIdx] = pre-tx snapshot
	mutSets   [][]MutationRecord // mutSets[txIdx] = mutations detected in tx
	txSetters []TxIndexSetter
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
	return &BlockObserver{
		writeSets: ws,
		trackers:  trackers,
		snapshots: snaps,
		mutSets:   make([][]MutationRecord, txCount),
		txSetters: setters,
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

// CaptureBeforeBlock snapshots all tracker states into the tx-0 slot. Call
// this immediately before the oracle FinalizeBlock to establish a baseline.
func (o *BlockObserver) CaptureBeforeBlock() {
	if len(o.trackers) == 0 || len(o.snapshots) == 0 {
		return
	}
	for i, t := range o.trackers {
		o.snapshots[0][i] = t.SnapshotOutOfKVStoreState()
	}
}

// CaptureAfterBlock diffs current tracker states against the tx-0 pre-block
// baseline and appends any changes to mutSets[0]. Call this immediately after
// oracle FinalizeBlock returns.
func (o *BlockObserver) CaptureAfterBlock() {
	if len(o.trackers) == 0 || len(o.mutSets) == 0 || len(o.snapshots) == 0 {
		return
	}
	for i, t := range o.trackers {
		after := t.SnapshotOutOfKVStoreState()
		before := o.snapshots[0][i]
		if !bytes.Equal(before, after) {
			o.mutSets[0] = append(o.mutSets[0], MutationRecord{
				Tracker: t.TrackerName(),
				Before:  before,
				After:   after,
			})
		}
	}
}
