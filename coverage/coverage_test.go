package coverage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/coverage"
)

var (
	entryA = coverage.Entry{Key: "mod-a", Module: "mod", MsgType: "MsgA", HandlerFn: "A"}
	entryB = coverage.Entry{Key: "mod-b", Module: "mod", MsgType: "MsgB", HandlerFn: "B"}
	entryC = coverage.Entry{Key: "mod-c", Module: "mod", MsgType: "MsgC", HandlerFn: "C"}
)

// withRegistry registers entries for the duration of a test and resets the
// registry on cleanup, ensuring each test starts from a clean slate.
func withRegistry(t *testing.T, entries ...coverage.Entry) {
	t.Helper()
	coverage.ClearRegistry()
	for _, e := range entries {
		coverage.Register(e.Key, e)
	}
	t.Cleanup(coverage.ClearRegistry)
}

func TestTracker_EmptyRun(t *testing.T) {
	withRegistry(t, entryA)

	r := coverage.NewTracker().Report()

	require.Empty(t, r.Covered)
	require.Len(t, r.Uncovered, 1)
	require.Equal(t, "mod-a", r.Uncovered[0].Key)
}

func TestTracker_SingleBlock(t *testing.T) {
	withRegistry(t, entryA, entryB)

	tr := coverage.NewTracker()
	tr.RecordBlock([]string{"mod-a", "mod-a"})

	r := tr.Report()
	require.Len(t, r.Covered, 1)
	require.Equal(t, "mod-a", r.Covered[0].Key)
	require.Equal(t, 2, r.Covered[0].Count)
	require.Len(t, r.Uncovered, 1)
	require.Equal(t, "mod-b", r.Uncovered[0].Key)
}

func TestTracker_MultiBlock(t *testing.T) {
	withRegistry(t, entryA, entryB)

	tr := coverage.NewTracker()
	tr.RecordBlock([]string{"mod-a"})
	tr.RecordBlock([]string{"mod-b", "mod-b"})
	tr.RecordBlock([]string{"mod-a", "mod-b"})

	r := tr.Report()
	require.Len(t, r.Covered, 2)
	require.Empty(t, r.Uncovered)
	require.Equal(t, "mod-a", r.Covered[0].Key)
	require.Equal(t, 2, r.Covered[0].Count)
	require.Equal(t, "mod-b", r.Covered[1].Key)
	require.Equal(t, 3, r.Covered[1].Count)
}

func TestTracker_UnregisteredKey(t *testing.T) {
	withRegistry(t, entryA)

	tr := coverage.NewTracker()
	// "unknown" is not in registry — must not appear in Report and must not panic
	tr.RecordBlock([]string{"unknown", "mod-a"})

	r := tr.Report()
	require.Len(t, r.Covered, 1)
	require.Equal(t, "mod-a", r.Covered[0].Key)
	require.Empty(t, r.Uncovered)
}

func TestTracker_AllCovered(t *testing.T) {
	withRegistry(t, entryA, entryB, entryC)

	tr := coverage.NewTracker()
	tr.RecordBlock([]string{"mod-a", "mod-b", "mod-c"})

	r := tr.Report()
	require.Len(t, r.Covered, 3)
	require.Empty(t, r.Uncovered)
}

func TestReport_SortedByKey(t *testing.T) {
	withRegistry(t, entryC, entryA, entryB)

	tr := coverage.NewTracker()
	tr.RecordBlock([]string{"mod-c", "mod-a"})

	r := tr.Report()
	require.Equal(t, "mod-a", r.Covered[0].Key)
	require.Equal(t, "mod-c", r.Covered[1].Key)
	require.Equal(t, "mod-b", r.Uncovered[0].Key)
}

func TestRegistered_SnapshotIsolation(t *testing.T) {
	withRegistry(t, entryA)

	snap1 := coverage.Registered()
	coverage.Register("mod-b", entryB)
	snap2 := coverage.Registered()

	require.NotEqual(t, len(snap1), len(snap2), "snapshots should differ after Register")
	require.Len(t, snap1, 1)
	require.Len(t, snap2, 2)
}
