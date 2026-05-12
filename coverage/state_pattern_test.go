package coverage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/coverage"
)

func TestStatePatternTracker_EmptyNoPatterns(t *testing.T) {
	tr := coverage.NewStatePatternTracker()
	r := tr.Report()
	require.Empty(t, r)
}

func TestStatePatternTracker_NilWriteSetsSkipped(t *testing.T) {
	tr := coverage.NewStatePatternTracker()
	tr.RecordBlock([]string{"bank-send"}, nil)
	r := tr.Report()
	require.Empty(t, r)
}

func TestStatePatternTracker_SameWriteSetOnePattern(t *testing.T) {
	tr := coverage.NewStatePatternTracker()
	keys := []string{"bank/aabbcc"}
	tr.RecordBlock([]string{"bank-send"}, [][]string{keys})
	tr.RecordBlock([]string{"bank-send"}, [][]string{keys})
	r := tr.Report()
	require.Len(t, r, 1)
	require.Equal(t, "bank-send", r[0].Key)
	require.Equal(t, 1, r[0].DistinctCount)
}

func TestStatePatternTracker_DifferentWriteSetsTwoPatterns(t *testing.T) {
	tr := coverage.NewStatePatternTracker()
	tr.RecordBlock([]string{"bank-send"}, [][]string{{"bank/aabb"}})
	tr.RecordBlock([]string{"bank-send"}, [][]string{{"bank/ccdd"}})
	r := tr.Report()
	require.Len(t, r, 1)
	require.Equal(t, 2, r[0].DistinctCount)
}

func TestStatePatternTracker_MultipleKeys(t *testing.T) {
	tr := coverage.NewStatePatternTracker()
	tr.RecordBlock(
		[]string{"bank-send", "staking-delegate"},
		[][]string{{"bank/aa"}, {"staking/bb"}},
	)
	r := tr.Report()
	require.Len(t, r, 2)
	require.Equal(t, "bank-send", r[0].Key)
	require.Equal(t, "staking-delegate", r[1].Key)
	require.Equal(t, 1, r[0].DistinctCount)
	require.Equal(t, 1, r[1].DistinctCount)
}

func TestStatePatternTracker_ReportSortedByKey(t *testing.T) {
	tr := coverage.NewStatePatternTracker()
	tr.RecordBlock(
		[]string{"zzz-msg", "aaa-msg"},
		[][]string{{"z/kv"}, {"a/kv"}},
	)
	r := tr.Report()
	require.Len(t, r, 2)
	require.Equal(t, "aaa-msg", r[0].Key)
	require.Equal(t, "zzz-msg", r[1].Key)
}

func TestStatePatternTracker_EmptyWriteSetCounted(t *testing.T) {
	tr := coverage.NewStatePatternTracker()
	// tx with no writes (nil slice within a non-nil writeSets)
	tr.RecordBlock([]string{"bank-send"}, [][]string{nil})
	tr.RecordBlock([]string{"bank-send"}, [][]string{nil})
	r := tr.Report()
	require.Len(t, r, 1)
	require.Equal(t, 1, r[0].DistinctCount, "nil write set is a valid (empty) pattern")
}

func TestStatePatternReport_DistinctCount(t *testing.T) {
	r := coverage.StatePatternReport{
		{Key: "bank-send", DistinctCount: 3},
		{Key: "staking-delegate", DistinctCount: 1},
	}
	require.Equal(t, 3, r.DistinctCount("bank-send"))
	require.Equal(t, 1, r.DistinctCount("staking-delegate"))
	require.Equal(t, 0, r.DistinctCount("unknown-key"))
}
