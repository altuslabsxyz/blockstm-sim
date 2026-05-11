package run

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/report"
)

// newExecutorFn is set by the sdk_hooks build to wire in a real executor.
// Nil in a public (no-tag) build, causing NewCommand to return an error.
var newExecutorFn func(probes int) Executor

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run fixture corpus and report comparison results",
		Long: "Execute all fixtures in a corpus directory and report comparison results.\n\n" +
			"With --probes=1 (default, F1 mode): compare oracle (sequential) vs a single BlockSTM probe.\n" +
			"With --probes=N (N>1, F2 mode): run 1 oracle + N BlockSTM probes with distinct scheduler\n" +
			"perturbations and compare all probes against each other to detect non-determinism.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if newExecutorFn == nil {
				return fmt.Errorf("simulation execution requires an SDK hook-enabled build")
			}

			corpus, _ := cmd.Flags().GetString("corpus")
			probes, _ := cmd.Flags().GetInt("probes")
			failOnDiv, _ := cmd.Flags().GetBool("fail-on-divergence")
			format, _ := cmd.Flags().GetString("format")

			stores, err := compare.LoadCorpusStores(corpus)
			if err != nil {
				return fmt.Errorf("load corpus: %w", err)
			}

			var rep report.Reporter
			switch format {
			case "json":
				rep = report.NewJSON(os.Stdout)
			case "markdown":
				rep = report.NewMarkdown(os.Stdout)
			default:
				rep = report.NewCLI(os.Stdout, os.Stderr)
			}

			cfg := Config{
				CorpusDir:        corpus,
				Probes:           probes,
				FailOnDivergence: failOnDiv,
			}

			code := RunHarness(cfg, newExecutorFn(probes), stores, rep, os.Stderr)
			if code != 0 {
				return fmt.Errorf("harness exited with code %d", code)
			}
			return nil
		},
	}

	cmd.Flags().String("corpus", "fixtures", "Fixture corpus directory")
	cmd.Flags().Int("probes", 1, "Number of probe variants")
	cmd.Flags().Bool("fail-on-divergence", false, "Exit 1 on any non-canary divergence")
	cmd.Flags().String("format", "text", "Output format: text, json, or markdown")

	return cmd
}
