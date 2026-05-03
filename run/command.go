package run

import (
	"os"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run fixture corpus and report comparison results",
		Long:  "Execute all fixtures in a corpus directory, comparing oracle (sequential) vs probe (BlockSTM) execution, and report results in human-readable format.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			corpus, _ := cmd.Flags().GetString("corpus")
			probes, _ := cmd.Flags().GetInt("probes")
			failOnDiv, _ := cmd.Flags().GetBool("fail-on-divergence")

			cfg := Config{
				CorpusDir:        corpus,
				Probes:           probes,
				FailOnDivergence: failOnDiv,
			}

			exec := NewFixtureExecutor()
			code := RunHarness(cfg, exec, os.Stdout, os.Stderr)
			if code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}

	cmd.Flags().String("corpus", "fixtures", "Fixture corpus directory")
	cmd.Flags().Int("probes", 1, "Number of probe variants")
	cmd.Flags().Bool("fail-on-divergence", false, "Exit 1 on any non-canary divergence")

	return cmd
}
