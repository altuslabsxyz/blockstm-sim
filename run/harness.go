package run

import (
	"fmt"
	"io"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/report"
)

type Executor interface {
	Init(genesis compare.GenesisSpec) error
	RunBlock(block compare.BlockSpec, height int64) (*compare.Result, error)
	Close()
}

type Config struct {
	CorpusDir        string
	Probes           int
	FailOnDivergence bool
}

func RunHarness(cfg Config, exec Executor, out, errOut io.Writer) int {
	rep := report.NewCLI(out, errOut)

	fixtures, err := compare.LoadCorpus(cfg.CorpusDir)
	if err != nil {
		fmt.Fprintf(errOut, "load corpus: %v\n", err)
		return 1
	}

	totalBlocks := 0
	for _, f := range fixtures {
		totalBlocks += len(f.Blocks)
	}

	rep.Header(cfg.CorpusDir, totalBlocks, cfg.Probes)

	var (
		okCount         int
		divergenceCount int
		canaryExpected  int
		canaryMissed    int
		blockNum        int
	)

	for _, fixture := range fixtures {
		if err := exec.Init(fixture.Genesis); err != nil {
			fmt.Fprintf(errOut, "init fixture %s: %v\n", fixture.Name, err)
			blockNum += len(fixture.Blocks)
			continue
		}

		for i, block := range fixture.Blocks {
			blockNum++
			result, err := exec.RunBlock(block, int64(i+1))
			if err != nil {
				fmt.Fprintf(errOut, "run block %d of %s: %v\n", i+1, fixture.Name, err)
				continue
			}

			outcome := report.BlockOutcome{
				Index:       blockNum,
				Total:       totalBlocks,
				FixtureName: fixture.Name,
				IsCanary:    fixture.IsCanary(),
				Verdict:     result.Verdict,
				Findings:    result.Findings,
			}
			rep.Block(outcome)

			switch {
			case fixture.IsCanary() && result.Verdict == compare.Divergence:
				canaryExpected++
			case fixture.IsCanary() && result.Verdict == compare.Match:
				canaryMissed++
			case result.Verdict == compare.Match:
				okCount++
			case result.Verdict == compare.Divergence:
				divergenceCount++
			}
		}

		exec.Close()
	}

	summary := report.Summary{
		TotalBlocks:     totalBlocks,
		OKCount:         okCount,
		DivergenceCount: divergenceCount,
		CanaryExpected:  canaryExpected,
		CanaryMissed:    canaryMissed,
		ReporterErrors:  rep.Errors(),
	}
	rep.Footer(summary, cfg.FailOnDivergence)

	return summary.ExitCode(cfg.FailOnDivergence)
}
