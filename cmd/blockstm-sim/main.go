package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/altuslabsxyz/blockstm-sim/cmd/extract"
	"github.com/altuslabsxyz/blockstm-sim/cmd/snapshot"
	"github.com/altuslabsxyz/blockstm-sim/detect"
	"github.com/altuslabsxyz/blockstm-sim/lint"
	"github.com/altuslabsxyz/blockstm-sim/run"
	"github.com/altuslabsxyz/blockstm-sim/version"

	_ "github.com/altuslabsxyz/blockstm-sim/cmd/blockstm-sim/sdkimpl"
	_ "github.com/altuslabsxyz/blockstm-sim/simharness"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "blockstm-sim",
		Short: "BlockSTM simulation and analysis tool",
		Long:  "BlockSTM simulation tool for detecting concurrency anomalies and analyzing parallel transaction execution.",
	}

	rootCmd.PersistentFlags().String("record-dir", ".run-records", "directory for NDJSON run records")
	rootCmd.PersistentFlags().Bool("record-off", false, "disable NDJSON run recording")

	rootCmd.AddCommand(version.NewCommand())
	rootCmd.AddCommand(run.NewCommand())
	rootCmd.AddCommand(extract.NewCommand())
	rootCmd.AddCommand(detect.NewCommand())
	rootCmd.AddCommand(lint.NewCommand())
	rootCmd.AddCommand(snapshot.NewCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
