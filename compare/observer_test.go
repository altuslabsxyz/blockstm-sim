//go:build sdk_hooks

package compare_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestBlockObserver_OnKVWrite(t *testing.T) {
	obs := compare.NewBlockObserver(3)

	obs.OnKVWrite("bank", []byte{0x01, 0x02}, 0)
	obs.OnKVWrite("bank", []byte{0x03, 0x04}, 0)
	obs.OnKVWrite("simcanary", []byte{0xAB}, 1)

	ws0 := obs.TxWriteSet(0)
	require.Equal(t, []string{"bank/0102", "bank/0304"}, ws0)

	ws1 := obs.TxWriteSet(1)
	require.Equal(t, []string{"simcanary/ab"}, ws1)

	ws2 := obs.TxWriteSet(2)
	require.Nil(t, ws2, "tx with no writes should return nil")
}

func TestBlockObserver_OnTxStart_Clears(t *testing.T) {
	obs := compare.NewBlockObserver(2)

	obs.OnKVWrite("bank", []byte{0x01}, 0)
	require.Len(t, obs.TxWriteSet(0), 1)

	obs.OnTxStart(0)
	require.Nil(t, obs.TxWriteSet(0), "OnTxStart must clear prior write set")
}

func TestBlockObserver_Dedup(t *testing.T) {
	obs := compare.NewBlockObserver(1)

	obs.OnKVWrite("bank", []byte{0x01}, 0)
	obs.OnKVWrite("bank", []byte{0x01}, 0)
	obs.OnKVWrite("bank", []byte{0x01}, 0)

	ws := obs.TxWriteSet(0)
	require.Equal(t, []string{"bank/01"}, ws, "duplicate keys must be deduplicated")
}

func TestBlockObserver_ConcurrentTxIndices(t *testing.T) {
	const txCount = 100
	obs := compare.NewBlockObserver(txCount)

	var wg sync.WaitGroup
	for i := 0; i < txCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			obs.OnKVWrite("bank", []byte{byte(idx)}, idx)
		}(i)
	}
	wg.Wait()

	for i := 0; i < txCount; i++ {
		ws := obs.TxWriteSet(i)
		require.Len(t, ws, 1, "each tx should have exactly one write")
	}
}

func TestBlockObserver_OutOfBounds(t *testing.T) {
	obs := compare.NewBlockObserver(2)

	obs.OnKVWrite("bank", []byte{0x01}, 99)
	obs.OnTxStart(99)
	require.Nil(t, obs.TxWriteSet(99))
}

// --- Mutation tracking tests ---

func TestBlockObserver_MutationTracking_DetectChange(t *testing.T) {
	tracker := &fakeTracker{name: "test.field", state: []byte("initial")}
	obs := compare.NewBlockObserver(2, tracker)

	obs.OnTxStart(0)
	tracker.state = []byte("mutated")
	obs.OnTxEnd(0, nil)

	muts := obs.TxMutations(0)
	require.Len(t, muts, 1)
	require.Equal(t, "test.field", muts[0].Tracker)
	require.Equal(t, []byte("initial"), muts[0].Before)
	require.Equal(t, []byte("mutated"), muts[0].After)
}

func TestBlockObserver_MutationTracking_NoChange(t *testing.T) {
	tracker := &fakeTracker{name: "test.field", state: []byte("same")}
	obs := compare.NewBlockObserver(2, tracker)

	obs.OnTxStart(0)
	// No mutation
	obs.OnTxEnd(0, nil)

	require.Nil(t, obs.TxMutations(0))
}

func TestBlockObserver_MutationTracking_MultiTx(t *testing.T) {
	tracker := &fakeTracker{name: "test.field", state: []byte("")}
	obs := compare.NewBlockObserver(3, tracker)

	obs.OnTxStart(0)
	tracker.state = []byte("a")
	obs.OnTxEnd(0, nil)

	obs.OnTxStart(1)
	tracker.state = []byte("ab")
	obs.OnTxEnd(1, nil)

	obs.OnTxStart(2)
	obs.OnTxEnd(2, nil)

	require.Len(t, obs.TxMutations(0), 1)
	require.Equal(t, []byte(""), obs.TxMutations(0)[0].Before)
	require.Equal(t, []byte("a"), obs.TxMutations(0)[0].After)

	require.Len(t, obs.TxMutations(1), 1)
	require.Equal(t, []byte("a"), obs.TxMutations(1)[0].Before)
	require.Equal(t, []byte("ab"), obs.TxMutations(1)[0].After)

	require.Nil(t, obs.TxMutations(2))
}

func TestBlockObserver_NoTrackers_BackwardCompat(t *testing.T) {
	obs := compare.NewBlockObserver(2)
	obs.OnTxStart(0)
	obs.OnTxEnd(0, nil)
	require.Nil(t, obs.TxMutations(0))
}
