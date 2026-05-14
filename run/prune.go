//go:build sdk_hooks

package run

import (
	"fmt"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// defaultPruneFactory is registered by chain-side code via RegisterDefaultPruneFactory
// so that the `blockstm-sim snapshot prune` CLI can operate without requiring an
// explicit factory parameter. Nil until registered.
var defaultPruneFactory AppFactory

// RegisterDefaultPruneFactory stores f as the factory returned by DefaultPruneFactory.
// Chain-side init() calls should invoke this before any CLI use of snapshot prune.
func RegisterDefaultPruneFactory(f AppFactory) { defaultPruneFactory = f }

// DefaultPruneFactory returns the factory registered via RegisterDefaultPruneFactory,
// or nil if none has been registered.
func DefaultPruneFactory() AppFactory { return defaultPruneFactory }

// PruneSnapshot permanently removes all IAVL versions above targetVersion from
// snapshotDir/application.db. It opens the DB in-place (no temp copy), loads
// the app at targetVersion via the supplied factory, calls RollbackToVersion to
// delete everything above, then closes the DB.
//
// This is a one-time, destructive operation. After it completes:
//   - application.db contains state only up to targetVersion.
//   - Subsequent InitFromState calls copy a smaller DB and the RollbackToVersion
//     call in loadAndTruncate becomes a near-no-op (nothing above targetVersion).
//   - blockstore.db is not modified; all block transactions remain available.
//
// factory must create an app bound to the supplied DB without calling
// LoadLatestVersion; loadAndTruncate handles version loading explicitly.
// This is the same contract as the factory passed to SnapshotExecutor.
func PruneSnapshot(snapshotDir string, targetVersion int64, factory AppFactory) error {
	db, err := openApplicationDB(snapshotDir)
	if err != nil {
		return fmt.Errorf("open application.db: %w", err)
	}
	// closeDB guards against double-close: the defer fires on error paths,
	// and the explicit db.Close() below runs on the success path.
	closeDB := true
	defer func() {
		if closeDB {
			_ = db.Close()
		}
	}()

	app, _, err := factory(db, nil)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}

	if err := loadAndTruncate(app, targetVersion); err != nil {
		return fmt.Errorf("prune to version %d: %w", targetVersion, err)
	}

	closeDB = false
	return db.Close()
}

// PruneSnapshotFromMeta computes targetVersion as meta.Start-1 (the IAVL
// version that InitFromState loads) and calls PruneSnapshot. This is the
// canonical one-time prune to call after `blockstm-sim extract`.
func PruneSnapshotFromMeta(snapshotDir string, meta compare.RangeMeta, factory AppFactory) error {
	return PruneSnapshot(snapshotDir, meta.Start-1, factory)
}
