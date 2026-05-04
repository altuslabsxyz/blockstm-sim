package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/x/simcanary/keeper"
)

func TestKeeper_TrackerName(t *testing.T) {
	k := keeper.NewKeeper(nil)
	require.Equal(t, "simcanary.sharedMap", k.TrackerName())
}

func TestKeeper_SnapshotOutOfKVStoreState_Empty(t *testing.T) {
	k := keeper.NewKeeper(nil)
	snap := k.SnapshotOutOfKVStoreState()
	require.Empty(t, snap, "fresh keeper has no out-of-KVStore state")
}

func TestKeeper_SnapshotOutOfKVStoreState_AfterSet(t *testing.T) {
	k := keeper.NewKeeper(nil)
	k.SetMapValue("key1", 42)
	k.SetMapValue("key2", 100)

	snap1 := k.SnapshotOutOfKVStoreState()
	require.NotEmpty(t, snap1)

	k.SetMapValue("key1", 99)
	snap2 := k.SnapshotOutOfKVStoreState()
	require.NotEqual(t, snap1, snap2, "snapshot must differ after mutation")
}

func TestKeeper_SnapshotOutOfKVStoreState_Deterministic(t *testing.T) {
	k := keeper.NewKeeper(nil)
	k.SetMapValue("b", 2)
	k.SetMapValue("a", 1)

	snap1 := k.SnapshotOutOfKVStoreState()
	snap2 := k.SnapshotOutOfKVStoreState()
	require.Equal(t, snap1, snap2, "same state must produce identical snapshots")

	// Verify sort order: "a" must come before "b" in the output.
	expected := "a=0000000000000001\nb=0000000000000002\n"
	require.Equal(t, []byte(expected), snap1, "keys must be sorted lexicographically")
}
