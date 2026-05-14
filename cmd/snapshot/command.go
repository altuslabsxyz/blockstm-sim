package snapshot

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// pruneSnapshotFn is populated by cmd/snapshot/command_sdk.go in an sdk_hooks
// build. Nil in a public (no-tag) build, causing the command to return an
// error explaining the build requirement.
var pruneSnapshotFn func(snapshotDir string, targetVersion int64) error

// NewCommand returns the parent "snapshot" command with all subcommands wired.
func NewCommand() *cobra.Command {
	parent := &cobra.Command{
		Use:   "snapshot",
		Short: "Snapshot management commands",
	}
	parent.AddCommand(newPruneCommand())
	return parent
}

func newPruneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove IAVL versions above the target from application.db (one-time, in-place)",
		Long: `Permanently removes all IAVL versions above --target-version from
snapshotDir/application.db. After pruning, subsequent test runs using
InitFromState copy a smaller DB and RollbackToVersion becomes a near-no-op.

This operation is destructive and irreversible. Run it once after
'blockstm-sim extract' before committing the snapshot to CI infrastructure.

Requires an SDK hook-enabled build with a registered factory
(chain side must call run.RegisterDefaultPruneFactory in an init()).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if pruneSnapshotFn == nil {
				return fmt.Errorf("snapshot prune requires an SDK hook-enabled build with a registered prune factory")
			}

			snapshotDir, _ := cmd.Flags().GetString("snapshot-dir")
			targetVersion, _ := cmd.Flags().GetInt64("target-version")

			if snapshotDir == "" {
				return fmt.Errorf("--snapshot-dir is required")
			}

			// If target-version was not supplied, infer meta.Start-1 from range.json.
			if targetVersion < 0 {
				meta, err := compare.LoadRangeMeta(snapshotDir)
				if err != nil {
					return fmt.Errorf("load range.json (needed to infer --target-version): %w", err)
				}
				targetVersion = meta.Start - 1
			}

			if err := pruneSnapshotFn(snapshotDir, targetVersion); err != nil {
				return fmt.Errorf("prune snapshot: %w", err)
			}

			_, err := fmt.Fprintf(cmd.OutOrStdout(),
				"Pruned %s/application.db to version %d\n", snapshotDir, targetVersion)
			return err
		},
	}

	cmd.Flags().String("snapshot-dir", "", "Path to the snapshot directory containing application.db")
	cmd.Flags().Int64("target-version", -1, "IAVL version to prune to (default: meta.Start-1 from range.json)")

	_ = cmd.MarkFlagRequired("snapshot-dir")

	return cmd
}
