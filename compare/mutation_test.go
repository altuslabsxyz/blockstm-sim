package compare_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// fakeTracker is a test double that implements compare.MutationTracker.
type fakeTracker struct {
	name  string
	state []byte
}

func (f *fakeTracker) TrackerName() string               { return f.name }
func (f *fakeTracker) SnapshotOutOfKVStoreState() []byte { return f.state }

// Verify fakeTracker satisfies the interface at compile time.
var _ compare.MutationTracker = (*fakeTracker)(nil)

func TestMutationRecord_Fields(t *testing.T) {
	rec := compare.MutationRecord{
		Tracker: "foo.bar",
		Before:  []byte("before"),
		After:   []byte("after"),
	}
	require.Equal(t, "foo.bar", rec.Tracker)
	require.Equal(t, []byte("before"), rec.Before)
	require.Equal(t, []byte("after"), rec.After)
}
