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
// Mutation tracking: designed for the oracle (sequential) runner only. Snapshots
// registered MutationTrackers at OnTxStart and diffs at OnTxEnd.
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
