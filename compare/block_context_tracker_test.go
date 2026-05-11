package compare_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestBlockContextTracker_Empty(t *testing.T) {
	tr := compare.NewBlockContextTracker(map[string]string{"height": "1"})
	require.Empty(t, tr.BlockContextMutations())
}

func TestBlockContextTracker_ReadNoWrite(t *testing.T) {
	tr := compare.NewBlockContextTracker(map[string]string{"height": "100"})
	tr.SetCurrentTx(0)
	got := tr.ReadField("height")
	require.Equal(t, "100", got)
	require.Empty(t, tr.BlockContextMutations(), "read alone must not generate a mutation")
}

func TestBlockContextTracker_WriteDetected(t *testing.T) {
	tr := compare.NewBlockContextTracker(map[string]string{"height": "100"})
	tr.SetCurrentTx(2)
	tr.WriteField("height", "999")
	muts := tr.BlockContextMutations()
	require.Len(t, muts, 1)
	require.Equal(t, "height", muts[0].Field)
	require.Equal(t, "100", muts[0].Before)
	require.Equal(t, "999", muts[0].After)
	require.Equal(t, 2, muts[0].WriterTx)
	require.Empty(t, muts[0].ReaderTxs)
}

func TestBlockContextTracker_ReadersTracked(t *testing.T) {
	tr := compare.NewBlockContextTracker(map[string]string{"height": "100"})
	tr.SetCurrentTx(0)
	tr.ReadField("height")
	tr.SetCurrentTx(1)
	tr.ReadField("height")
	tr.SetCurrentTx(3)
	tr.WriteField("height", "200")
	muts := tr.BlockContextMutations()
	require.Len(t, muts, 1)
	require.Equal(t, []int{0, 1}, muts[0].ReaderTxs)
	require.Equal(t, 3, muts[0].WriterTx)
}

func TestBlockContextTracker_DuplicateReadDeduped(t *testing.T) {
	tr := compare.NewBlockContextTracker(map[string]string{"height": "1"})
	tr.SetCurrentTx(0)
	tr.ReadField("height")
	tr.ReadField("height")
	tr.SetCurrentTx(1)
	tr.WriteField("height", "2")
	muts := tr.BlockContextMutations()
	require.Equal(t, []int{0}, muts[0].ReaderTxs, "same tx reading twice must be deduplicated")
}

func TestBlockContextTracker_MultipleFields(t *testing.T) {
	tr := compare.NewBlockContextTracker(map[string]string{"height": "1", "chain_id": "test"})
	tr.SetCurrentTx(0)
	tr.ReadField("chain_id")
	tr.SetCurrentTx(1)
	tr.WriteField("height", "2")
	tr.WriteField("chain_id", "mutated")
	muts := tr.BlockContextMutations()
	require.Len(t, muts, 2)

	var chainMut compare.BlockContextMutation
	for _, m := range muts {
		if m.Field == "chain_id" {
			chainMut = m
		}
	}
	require.Equal(t, []int{0}, chainMut.ReaderTxs)
}

// Regression test for #46: readers must be reset after each write so that a
// subsequent mutation only records readers since the previous write.
func TestBlockContextTracker_WriteField_ReadersResetAfterWrite(t *testing.T) {
	tr := compare.NewBlockContextTracker(map[string]string{"height": "1"})

	// tx0 reads, then tx1 writes, then tx2 reads, then tx3 writes again.
	tr.SetCurrentTx(0)
	tr.ReadField("height")

	tr.SetCurrentTx(1)
	tr.WriteField("height", "2") // first write — readers should be [0]

	tr.SetCurrentTx(2)
	tr.ReadField("height")

	tr.SetCurrentTx(3)
	tr.WriteField("height", "3") // second write — readers should be [2] only, not [0,2]

	muts := tr.BlockContextMutations()
	require.Len(t, muts, 2)

	require.Equal(t, []int{0}, muts[0].ReaderTxs, "first mutation: only tx0 read before write")
	require.Equal(t, []int{2}, muts[1].ReaderTxs, "second mutation: only tx2 read since last write")
}

func TestBlockContextTracker_Reset(t *testing.T) {
	tr := compare.NewBlockContextTracker(map[string]string{"height": "1"})
	tr.SetCurrentTx(0)
	tr.WriteField("height", "999")
	require.Len(t, tr.BlockContextMutations(), 1)

	tr.Reset(map[string]string{"height": "2"})
	require.Empty(t, tr.BlockContextMutations(), "Reset must clear mutations")
	tr.SetCurrentTx(0)
	got := tr.ReadField("height")
	require.Equal(t, "2", got)
}
