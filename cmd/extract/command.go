package extract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	cmtdb "github.com/cometbft/cometbft-db"
	cmtstore "github.com/cometbft/cometbft/store"
	"github.com/spf13/cobra"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Create a snapshot corpus descriptor from a node data directory",
		Long: `Validates that the given height range exists in the source blockstore,
then writes a range.json corpus descriptor to the output directory.

After running extract, copy or symlink blockstore.db and application.db
from the source data directory into the output directory before running
blockstm-sim with the snapshot corpus.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sourceDir, _ := cmd.Flags().GetString("source")
			outDir, _ := cmd.Flags().GetString("out")
			start, _ := cmd.Flags().GetInt64("start")
			end, _ := cmd.Flags().GetInt64("end")
			chainID, _ := cmd.Flags().GetString("chain-id")
			bondDenom, _ := cmd.Flags().GetString("bond-denom")
			appVersion, _ := cmd.Flags().GetUint64("app-version")

			if start <= 0 || end <= 0 || end < start {
				return fmt.Errorf("invalid height range: start=%d end=%d", start, end)
			}

			if err := validateBlockstore(sourceDir, start, end); err != nil {
				return fmt.Errorf("validate blockstore: %w", err)
			}

			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("create output dir: %w", err)
			}

			meta := compare.RangeMeta{
				ChainID:    chainID,
				AppVersion: appVersion,
				Start:      start,
				End:        end,
				BondDenom:  bondDenom,
			}
			if err := writeRangeMeta(outDir, meta); err != nil {
				return fmt.Errorf("write range.json: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"range.json written to %s\n\nNext steps:\n  cp -r %s/blockstore.db %s/\n  cp -r %s/application.db %s/\n",
				outDir, sourceDir, outDir, sourceDir, outDir)
			return nil
		},
	}

	cmd.Flags().String("source", "", "Node data directory containing blockstore.db and application.db")
	cmd.Flags().String("out", "", "Output corpus directory")
	cmd.Flags().Int64("start", 0, "First block height to include")
	cmd.Flags().Int64("end", 0, "Last block height to include")
	cmd.Flags().String("chain-id", "", "Chain ID (e.g. cosmoshub-4)")
	cmd.Flags().String("bond-denom", "", "Bond denomination (e.g. uatom)")
	cmd.Flags().Uint64("app-version", 0, "App version from consensus params")

	_ = cmd.MarkFlagRequired("source")
	_ = cmd.MarkFlagRequired("out")
	_ = cmd.MarkFlagRequired("start")
	_ = cmd.MarkFlagRequired("end")
	_ = cmd.MarkFlagRequired("chain-id")
	_ = cmd.MarkFlagRequired("bond-denom")

	return cmd
}

func validateBlockstore(dir string, start, end int64) error {
	db, err := cmtdb.NewGoLevelDB("blockstore", dir)
	if err != nil {
		return fmt.Errorf("open blockstore.db in %s: %w", dir, err)
	}
	defer db.Close()

	bs := cmtstore.NewBlockStore(db)
	if bs.Base() > start {
		return fmt.Errorf("blockstore base height %d is above requested start %d", bs.Base(), start)
	}
	if bs.Height() < end {
		return fmt.Errorf("blockstore height %d is below requested end %d", bs.Height(), end)
	}
	if bs.LoadBlock(start) == nil {
		return fmt.Errorf("block %d not found in blockstore", start)
	}
	if bs.LoadBlock(end) == nil {
		return fmt.Errorf("block %d not found in blockstore", end)
	}
	return nil
}

func writeRangeMeta(dir string, meta compare.RangeMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "range.json"), append(data, '\n'), 0o644)
}
