package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/altuslabsxyz/blockstm-sim/version"

	_ "github.com/altuslabsxyz/blockstm-sim/simharness"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "blockstm-sim",
		Short: "BlockSTM simulation and analysis tool",
		Long:  "BlockSTM simulation tool for detecting concurrency anomalies and analyzing parallel transaction execution.",
	}

	rootCmd.AddCommand(version.NewCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
