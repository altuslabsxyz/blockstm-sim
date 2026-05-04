package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/report"
)

func TestMarkdownReporter_Title(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("fixtures", 5, 1)
	r.Footer(report.Summary{TotalBlocks: 5, OKCount: 5}, false)

	out := buf.String()
	require.Contains(t, out, "# BlockSTM Sim — Run Report")
	require.Contains(t, out, "**Corpus:** fixtures")
	require.Contains(t, out, "**Probes:** 1")
}

func TestMarkdownReporter_SummaryTable(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("fixtures", 5, 1)
	r.Footer(report.Summary{
		TotalBlocks:     5,
		OKCount:         3,
		DivergenceCount: 1,
		CanaryExpected:  1,
		CanaryMissed:    0,
	}, false)

	out := buf.String()
	require.Contains(t, out, "## Summary")
	require.Contains(t, out, "| Total Blocks | 5 |")
	require.Contains(t, out, "| OK | 3 |")
	require.Contains(t, out, "| Divergences | 1 |")
	require.Contains(t, out, "| Canary Expected | 1 |")
	require.Contains(t, out, "| Canary Missed | 0 |")
}

func TestMarkdownReporter_DivergencesSection(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("fixtures", 2, 1)
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       2,
		FixtureName: "canary-01-keeper",
		IsCanary:    true,
		Verdict:     compare.Divergence,
		Findings: []compare.Finding{
			{
				ID:         "abc123",
				Height:     1,
				TxIndex:    -1,
				ProbeIndex: 0,
				Dimension:  compare.DimAppHash,
			},
		},
	})
	r.Footer(report.Summary{TotalBlocks: 2, CanaryExpected: 1}, false)

	out := buf.String()
	require.Contains(t, out, "## Divergences")
	require.Contains(t, out, "### canary-01-keeper")
	require.Contains(t, out, "`abc123`")
	require.Contains(t, out, "app_hash")
}

func TestMarkdownReporter_MissedCanaries(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("fixtures", 1, 1)
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       1,
		FixtureName: "canary-02-missed",
		IsCanary:    true,
		Verdict:     compare.Match,
	})
	r.Footer(report.Summary{TotalBlocks: 1, CanaryMissed: 1}, false)

	out := buf.String()
	require.Contains(t, out, "## Missed Canaries")
	require.Contains(t, out, "canary-02-missed")
}

func TestMarkdownReporter_ReproduceCommand_Defaults(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("fixtures", 0, 1)
	r.Footer(report.Summary{}, false)

	out := buf.String()
	require.Contains(t, out, "## Reproduce")
	require.Contains(t, out, "blockstm-sim run")
	require.NotContains(t, out, "--corpus")
	require.NotContains(t, out, "--probes")
}

func TestMarkdownReporter_ReproduceCommand_NonDefaults(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("my-corpus", 0, 3)
	r.Footer(report.Summary{}, true)

	out := buf.String()
	require.Contains(t, out, "--corpus my-corpus")
	require.Contains(t, out, "--probes 3")
	require.Contains(t, out, "--fail-on-divergence")
}

func TestMarkdownReporter_NoDivergencesSection_WhenAllOK(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("fixtures", 2, 1)
	r.Block(report.BlockOutcome{Index: 1, Total: 2, FixtureName: "ok-01", Verdict: compare.Match})
	r.Block(report.BlockOutcome{Index: 2, Total: 2, FixtureName: "ok-02", Verdict: compare.Match})
	r.Footer(report.Summary{TotalBlocks: 2, OKCount: 2}, false)

	out := buf.String()
	require.False(t, strings.Contains(out, "## Divergences"), "no divergences section expected")
}

func TestMarkdownReporter_Errors(t *testing.T) {
	r := report.NewMarkdown(&bytes.Buffer{})
	require.Equal(t, 0, r.Errors())
}
