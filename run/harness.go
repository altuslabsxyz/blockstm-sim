package run

import (
	"context"
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

func RunHarness(cfg Config, exec Executor, stores []compare.CorpusStore, out, errOut io.Writer) int {
	rep := report.NewCLI(out, errOut)

	totalBlocks := 0
	for _, s := range stores {
		totalBlocks += s.BlockCount()
	}

	rep.Header(cfg.CorpusDir, totalBlocks, cfg.Probes)

	ctx := context.Background()

	var (
		okCount         int
		divergenceCount int
		canaryExpected  int
		canaryMissed    int
		blockNum        int
	)

	for _, store := range stores {
		name := store.Name()
		isCanary := store.IsCanary()

		if err := exec.Init(store.Genesis()); err != nil {
			fmt.Fprintf(errOut, "init fixture %s: %v\n", name, err)
			blockNum += store.BlockCount()
			continue
		}

		var localHeight int64
		for block, err := range store.Iter(ctx) {
			if err != nil {
				fmt.Fprintf(errOut, "iter fixture %s: %v\n", name, err)
				break
			}
			localHeight++
			blockNum++
			effectiveHeight := localHeight
			if block.Height > 0 {
				effectiveHeight = block.Height
			}

			result, err := exec.RunBlock(block, effectiveHeight)
			if err != nil {
				fmt.Fprintf(errOut, "run block %d of %s: %v\n", effectiveHeight, name, err)
				continue
			}

			outcome := report.BlockOutcome{
				Index:       blockNum,
				Total:       totalBlocks,
				FixtureName: name,
				IsCanary:    isCanary,
				Verdict:     result.Verdict,
				Findings:    result.Findings,
			}
			rep.Block(outcome)

			switch {
			case isCanary && result.Verdict == compare.Divergence:
				canaryExpected++
			case isCanary && result.Verdict == compare.Match:
				canaryMissed++
			case result.Verdict == compare.Match:
				okCount++
			case result.Verdict == compare.Divergence:
				divergenceCount++
			}
		}

		store.Close()
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
