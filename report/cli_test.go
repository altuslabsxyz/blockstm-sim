package report_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/coverage"
	"github.com/altuslabsxyz/blockstm-sim/report"
)

func newReporter() (*report.CLIReporter, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return report.NewCLI(out, errOut), out, errOut
}

func TestHeader(t *testing.T) {
	r, out, _ := newReporter()
	r.Header("fixtures", 20, 1)
	require.Equal(t, "Harness  corpus=fixtures  blocks=20  probes=1\n\n", out.String())
}

func TestBlock_OK(t *testing.T) {
	r, out, _ := newReporter()
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       20,
		FixtureName: "01-single-bank-send",
		Verdict:     compare.Match,
	})
	require.Equal(t, "[ 1/20] ok 01-single-bank-send\n", out.String())
}

func TestBlock_OK_ThreeDigits(t *testing.T) {
	r, out, _ := newReporter()
	r.Block(report.BlockOutcome{
		Index:       3,
		Total:       100,
		FixtureName: "fixture-x",
		Verdict:     compare.Match,
	})
	require.Equal(t, "[  3/100] ok fixture-x\n", out.String())
}

func TestBlock_Divergence(t *testing.T) {
	r, out, _ := newReporter()
	r.Block(report.BlockOutcome{
		Index:       3,
		Total:       20,
		FixtureName: "canary-01-keeper-map",
		IsCanary:    true,
		Verdict:     compare.Divergence,
		Findings: []compare.Finding{
			{
				Dimension:  compare.DimAppHash,
				Oracle:     "abc123ef0011223344",
				Probe:      "def4567a0099887766",
				ProbeIndex: 0,
			},
		},
	})
	got := out.String()
	require.Contains(t, got, "[ 3/20] DIVERGENCE canary-01-keeper-map\n")
	require.Contains(t, got, "dimension  : app_hash")
	require.Contains(t, got, "oracle     : 0xabc123ef00...")
	require.Contains(t, got, "candidate 0: 0xdef4567a00...")
	require.Contains(t, got, "details    : post-block app hash mismatch")
}

func TestBlock_CanaryMissed(t *testing.T) {
	r, out, _ := newReporter()
	r.Block(report.BlockOutcome{
		Index:       4,
		Total:       20,
		FixtureName: "canary-01-keeper-map",
		IsCanary:    true,
		Verdict:     compare.Match,
	})
	require.Equal(t, "[ 4/20] CANARY MISSED canary-01-keeper-map\n", out.String())
}

func TestFooter(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{
		TotalBlocks:     20,
		OKCount:         18,
		DivergenceCount: 1,
		CanaryExpected:  1,
		CanaryMissed:    1,
	}, false)
	got := out.String()
	require.Contains(t, got, "Summary")
	require.Contains(t, got, "20 blocks run / 18 ok / 1 divergence (1 canary expected) / 1 canary missed")
	require.Contains(t, got, "Exit: 1")
}

func TestFooter_ExitZero(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{
		TotalBlocks: 5,
		OKCount:     5,
	}, false)
	require.Contains(t, out.String(), "Exit: 0")
}

func TestFooter_FailOnDivergence(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{
		TotalBlocks:     5,
		OKCount:         4,
		DivergenceCount: 1,
	}, true)
	require.Contains(t, out.String(), "Exit: 1")
}

func TestSummary_ExitCode(t *testing.T) {
	tests := []struct {
		name             string
		summary          report.Summary
		failOnDivergence bool
		want             int
	}{
		{"all ok", report.Summary{OKCount: 5}, false, 0},
		{"divergence no flag", report.Summary{DivergenceCount: 1}, false, 0},
		{"divergence with flag", report.Summary{DivergenceCount: 1}, true, 1},
		{"canary missed always 1", report.Summary{CanaryMissed: 1}, false, 1},
		{"canary expected ok", report.Summary{CanaryExpected: 1}, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.summary.ExitCode(tt.failOnDivergence))
		})
	}
}

func TestFooter_Coverage_NoEntries(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{TotalBlocks: 5, OKCount: 5}, false)
	got := out.String()
	require.NotContains(t, got, "Coverage")
}

func TestFooter_Coverage_AllCovered(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{
		TotalBlocks: 2,
		OKCount:     2,
		Coverage: coverage.Report{
			Covered: []coverage.EntryStat{
				{Entry: coverage.Entry{Key: "bank-send", Module: "bank", MsgType: "MsgSend", HandlerFn: "Send"}, Count: 5},
			},
		},
	}, false)
	got := out.String()
	require.Contains(t, got, "Coverage  registered=1  covered=1  uncovered=0")
	require.Contains(t, got, "bank")
	require.Contains(t, got, "MsgSend")
	require.Contains(t, got, "5 tx")
	require.NotContains(t, got, "!")
}

func TestFooter_Coverage_SomeUncovered(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{
		TotalBlocks: 1,
		OKCount:     1,
		Coverage: coverage.Report{
			Covered: []coverage.EntryStat{
				{Entry: coverage.Entry{Key: "bank-send", Module: "bank", MsgType: "MsgSend", HandlerFn: "Send"}, Count: 3},
			},
			Uncovered: []coverage.Entry{
				{Key: "canary-map-set", Module: "simcanary", MsgType: "MsgCanaryMapSet", HandlerFn: "MapSet"},
			},
		},
	}, false)
	got := out.String()
	require.Contains(t, got, "Coverage  registered=2  covered=1  uncovered=1")
	require.Contains(t, got, "! simcanary")
	require.Contains(t, got, "MapSet")
	require.Contains(t, got, "0 tx")
}

func TestFooter_Coverage_StatePatterns(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{
		TotalBlocks: 3,
		OKCount:     3,
		Coverage: coverage.Report{
			Covered: []coverage.EntryStat{
				{Entry: coverage.Entry{Key: "bank-send", Module: "bank", MsgType: "MsgSend", HandlerFn: "Send"}, Count: 3},
			},
		},
		StatePatterns: coverage.StatePatternReport{
			{Key: "bank-send", DistinctCount: 2},
		},
	}, false)
	got := out.String()
	require.Contains(t, got, "3 tx")
	require.Contains(t, got, "2 state patterns")
}

func TestFooter_Coverage_NoStatePatterns(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{
		TotalBlocks: 1,
		OKCount:     1,
		Coverage: coverage.Report{
			Covered: []coverage.EntryStat{
				{Entry: coverage.Entry{Key: "bank-send", Module: "bank", MsgType: "MsgSend", HandlerFn: "Send"}, Count: 1},
			},
		},
		// StatePatterns intentionally empty — should not appear in output
	}, false)
	got := out.String()
	require.Contains(t, got, "1 tx")
	require.NotContains(t, got, "state patterns")
}

type errWriter struct{ n int }

func (w *errWriter) Write([]byte) (int, error) { w.n++; return 0, &writeErr{} }

type writeErr struct{}

func (e *writeErr) Error() string { return "write failed" }

func TestReporterErrorCounting(t *testing.T) {
	ew := &errWriter{}
	errBuf := &bytes.Buffer{}
	r := report.NewCLI(ew, errBuf)

	r.Header("fixtures", 5, 1)
	r.Block(report.BlockOutcome{Index: 1, Total: 5, FixtureName: "f", Verdict: compare.Match})

	require.Equal(t, 2, r.Errors())
	require.Contains(t, errBuf.String(), "reporter: write error")
}
