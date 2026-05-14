//go:build sdk_hooks

package snapshot

import (
	"fmt"

	"github.com/altuslabsxyz/blockstm-sim/run"
)

func init() {
	pruneSnapshotFn = func(snapshotDir string, targetVersion int64) error {
		factory := run.DefaultPruneFactory()
		if factory == nil {
			return fmt.Errorf("no prune factory registered; chain side must call run.RegisterDefaultPruneFactory in an init()")
		}
		return run.PruneSnapshot(snapshotDir, targetVersion, factory)
	}
}
