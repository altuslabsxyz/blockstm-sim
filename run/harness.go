package run

import (
	"context"
	"fmt"
	"io"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/coverage"
	"github.com/altuslabsxyz/blockstm-sim/report"
)

type Executor interface {
	Init(genesis compare.GenesisSpec) error
	RunBlock(block compare.BlockSpec, height int64) (*compare.Result, error)
	Close()
}

// ConflictDetectionSetter is an optional interface for executors that can
// toggle installation of the probe conflict observer. RunHarness enables it
// only when hot-key reporting is active (Config.HotKeyMinTxs > 0), so a
// disabled flag also disables collection, not just display.
type ConflictDetectionSetter interface {
	SetConflictDetection(enabled bool)
}

// StateInitializer is an optional interface for executors that can initialise
// from an existing snapshot directory rather than from a genesis spec.
// The executor is responsible for opening application.db handles from
// snapshotDir (oracle and probe need physically separate stores) and for
// loading the IAVL version at meta.Start - 1 so that the next FinalizeBlock
// at height meta.Start is accepted.
type StateInitializer interface {
	InitFromState(snapshotDir string, meta compare.RangeMeta) error
}

type Config struct {
	CorpusDir        string
	Probes           int
	FailOnDivergence bool

	// HotKeyMinTxs reports a probe conflict key only when at least this many
	// distinct transactions re-executed because of it. <= 0 disables hot-key
	// reporting. Performance diagnostics only — never affects the exit code.
	// The execution ratio has no threshold: it is always reported when the
	// probe supplies execution stats.
	HotKeyMinTxs int
}

func RunHarness(cfg Config, exec Executor, stores []compare.CorpusStore, rep report.Reporter, errOut io.Writer) int {
	totalBlocks := 0
	for _, s := range stores {
		totalBlocks += s.BlockCount()
	}

	rep.Header(cfg.CorpusDir, totalBlocks, cfg.Probes)

	if cd, ok := exec.(ConflictDetectionSetter); ok {
		cd.SetConflictDetection(cfg.HotKeyMinTxs > 0)
	}

	ctx := context.Background()

	tracker := coverage.NewTracker()
	patternTracker := coverage.NewStatePatternTracker()

	var (
		okCount         int
		divergenceCount int
		canaryExpected  int
		canaryMissed    int
		blockNum        int

		hotKeyBlocks int
		maxExecRatio float64
	)

	for _, store := range stores {
		name := store.Name()
		isCanary := store.IsCanary()

		if snapshotDir := store.SnapshotDir(); snapshotDir != "" {
			si, ok := exec.(StateInitializer)
			if !ok {
				_, _ = fmt.Fprintf(errOut, "executor does not support state-based init for %s\n", name)
				_ = store.Close()
				blockNum += store.BlockCount()
				continue
			}
			if err := si.InitFromState(snapshotDir, store.Meta()); err != nil {
				_, _ = fmt.Fprintf(errOut, "init from state %s: %v\n", name, err)
				_ = store.Close()
				blockNum += store.BlockCount()
				continue
			}
		} else {
			if err := exec.Init(store.Genesis()); err != nil {
				_, _ = fmt.Fprintf(errOut, "init fixture %s: %v\n", name, err)
				_ = store.Close()
				blockNum += store.BlockCount()
				continue
			}
		}

		var localHeight int64
		for block, err := range store.Iter(ctx) {
			if err != nil {
				_, _ = fmt.Fprintf(errOut, "iter fixture %s: %v\n", name, err)
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
				_, _ = fmt.Fprintf(errOut, "run block %d of %s: %v\n", effectiveHeight, name, err)
				continue
			}

			tracker.RecordBlock(result.MsgKeys)
			patternTracker.RecordBlock(result.MsgKeys, result.TxWriteSets)

			outcome := report.BlockOutcome{
				Index:          blockNum,
				Total:          totalBlocks,
				FixtureName:    name,
				IsCanary:       isCanary,
				Verdict:        result.Verdict,
				Findings:       result.Findings,
				OracleTxCodes:  result.OracleTxCodes,
				ExecutionRatio: result.ExecutionRatio,
			}
			if cfg.HotKeyMinTxs > 0 {
				for _, hk := range result.HotKeys {
					if len(hk.Txs) >= cfg.HotKeyMinTxs {
						outcome.HotKeys = append(outcome.HotKeys, hk)
					}
				}
			}
			rep.Block(outcome)

			if len(outcome.HotKeys) > 0 {
				hotKeyBlocks++
			}
			if result.ExecutionRatio > maxExecRatio {
				maxExecRatio = result.ExecutionRatio
			}

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

		_ = store.Close()
		exec.Close()
	}

	// When every canary block is missed it almost always means a systematic
	// setup error rather than a true detection failure. Surface actionable hints
	// before the summary so the operator does not have to guess.
	if canaryMissed > 0 && canaryExpected == 0 {
		_, _ = fmt.Fprintf(errOut, "\nWARN: all canary blocks missed — possible causes:\n")
		_, _ = fmt.Fprintf(errOut, "  • genesis mismatch: factory genesis ≠ fixture genesis\n")
		_, _ = fmt.Fprintf(errOut, "    → use WithAppFactoryFunc instead of WithAppFactory so each fixture\n")
		_, _ = fmt.Fprintf(errOut, "      gets an app bootstrapped from its own accounts\n")
		_, _ = fmt.Fprintf(errOut, "  • ante handler rejection: canary message types blocked before handler runs\n")
		_, _ = fmt.Fprintf(errOut, "    → check oracle TxResult error_code for canary blocks\n")
		_, _ = fmt.Fprintf(errOut, "  • keeper not registered via RegisterKeeperDiscovery\n")
		_, _ = fmt.Fprintf(errOut, "    → check sdkimpl.go RegisterKeeperDiscovery callback\n\n")
	}

	summary := report.Summary{
		TotalBlocks:     totalBlocks,
		OKCount:         okCount,
		DivergenceCount: divergenceCount,
		CanaryExpected:  canaryExpected,
		CanaryMissed:    canaryMissed,
		ReporterErrors:  rep.Errors(),
		Coverage:        tracker.Report(),
		StatePatterns:   patternTracker.Report(),
		HotKeyBlocks:    hotKeyBlocks,
		MaxExecRatio:    maxExecRatio,
	}
	rep.Footer(summary, cfg.FailOnDivergence)

	return summary.ExitCode(cfg.FailOnDivergence)
}
