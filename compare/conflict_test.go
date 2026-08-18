package compare_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestConflictSink_RecordAndDrain(t *testing.T) {
	s := compare.NewConflictSink()
	s.RecordConflict(compare.ConflictRecord{Store: "acc", TxIndex: 1, Key: []byte{0xaa}, Reason: "version"})
	s.RecordConflict(compare.ConflictRecord{Store: "acc", TxIndex: 2, Key: []byte{0xaa}, Reason: "version"})
	s.RecordStats(compare.ExecutionStats{BlockSize: 4, ExecutedTxns: 6, ValidatedTxns: 7})

	records, stats := s.Drain()
	require.Len(t, records, 2)
	require.NotNil(t, stats)
	require.Equal(t, 4, stats.BlockSize)
	require.Equal(t, int64(6), stats.ExecutedTxns)

	// Drain resets the sink.
	records, stats = s.Drain()
	require.Nil(t, records)
	require.Nil(t, stats)
}

func TestConflictSink_ConcurrentRecord(t *testing.T) {
	s := compare.NewConflictSink()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(tx int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				s.RecordConflict(compare.ConflictRecord{Store: "acc", TxIndex: tx, Key: []byte{0x01}})
			}
		}(i)
	}
	wg.Wait()

	records, _ := s.Drain()
	require.Len(t, records, 800)
}

func TestExecutionStats_Ratio(t *testing.T) {
	require.Equal(t, 0.0, compare.ExecutionStats{}.Ratio())
	require.Equal(t, 1.0, compare.ExecutionStats{BlockSize: 5, ExecutedTxns: 5}.Ratio())
	require.Equal(t, 3.0, compare.ExecutionStats{BlockSize: 2, ExecutedTxns: 6}.Ratio())
}

func TestAggregateHotKeys_Empty(t *testing.T) {
	require.Nil(t, compare.AggregateHotKeys(nil))
}

func TestAggregateHotKeys_GroupsAndSorts(t *testing.T) {
	records := []compare.ConflictRecord{
		{Store: "acc", TxIndex: 3, Key: []byte{0xaa}, Reason: "version"},
		{Store: "acc", TxIndex: 1, Key: []byte{0xaa}, Reason: "version"},
		{Store: "acc", TxIndex: 1, Key: []byte{0xaa}, Reason: "version"}, // same tx, counted once in Txs
		{Store: "bank", TxIndex: 2, Key: []byte{0xbb}, Reason: "version"},
	}

	hot := compare.AggregateHotKeys(records)
	require.Len(t, hot, 2)

	// Sorted by Conflicts descending: acc/aa (3 records) first.
	require.Equal(t, "acc", hot[0].Store)
	require.Equal(t, "aa", hot[0].Key)
	require.Equal(t, 3, hot[0].Conflicts)
	require.Equal(t, []int{1, 3}, hot[0].Txs) // distinct, sorted

	require.Equal(t, "bank", hot[1].Store)
	require.Equal(t, "bb", hot[1].Key)
	require.Equal(t, 1, hot[1].Conflicts)
	require.Equal(t, []int{2}, hot[1].Txs)
}

func TestAggregateHotKeys_SameKeyDifferentStore(t *testing.T) {
	records := []compare.ConflictRecord{
		{Store: "acc", TxIndex: 0, Key: []byte{0x01}},
		{Store: "bank", TxIndex: 0, Key: []byte{0x01}},
	}
	hot := compare.AggregateHotKeys(records)
	require.Len(t, hot, 2, "same key bytes in different stores must not merge")
}
