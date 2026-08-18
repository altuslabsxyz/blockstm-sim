package report_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/report"
)

func TestCLIBlock_PerfHotKeys(t *testing.T) {
	r, out, _ := newReporter()
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       5,
		FixtureName: "evm-parallel",
		Verdict:     compare.Match,
		HotKeys: []compare.HotKeyStat{
			{Store: "acc", Key: "01aabbccddeeff", Conflicts: 12, Txs: []int{1, 2, 3, 4}},
		},
		ExecutionRatio: 3.4,
	})
	got := out.String()
	require.Contains(t, got, "ok evm-parallel")
	require.Contains(t, got, "exec-ratio 3.40")
	require.Contains(t, got, "hot key acc/0x01aabbccdd...")
	require.Contains(t, got, "conflicts=12")
	require.Contains(t, got, "txs=[1,2,3,4]")
}

func TestCLIBlock_ExecRatioAlwaysReported(t *testing.T) {
	r, out, _ := newReporter()
	// The ratio is reported whenever stats are available — no threshold.
	r.Block(report.BlockOutcome{
		Index:          1,
		Total:          5,
		FixtureName:    "clean",
		Verdict:        compare.Match,
		ExecutionRatio: 1.0,
	})
	require.Contains(t, out.String(), "exec-ratio 1.00")
}

func TestCLIBlock_PerfSilentWithoutStats(t *testing.T) {
	r, out, _ := newReporter()
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       5,
		FixtureName: "no-stats",
		Verdict:     compare.Match,
		// ExecutionRatio 0 = stats unavailable → nothing extra printed.
	})
	require.Equal(t, "[1/5] ok no-stats\n", out.String())
}

func TestCLIBlock_PerfTxListTruncation(t *testing.T) {
	r, out, _ := newReporter()
	txs := make([]int, 12)
	for i := range txs {
		txs[i] = i
	}
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       1,
		FixtureName: "many",
		Verdict:     compare.Match,
		HotKeys:     []compare.HotKeyStat{{Store: "acc", Key: "aa", Conflicts: 12, Txs: txs}},
	})
	require.Contains(t, out.String(), "…(12 total)")
}

func TestCLIFooter_PerfLine(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{
		TotalBlocks:  3,
		OKCount:      3,
		HotKeyBlocks: 2,
		MaxExecRatio: 3.4,
	}, false)
	got := out.String()
	require.Contains(t, got, "Perf  max-exec-ratio=3.40  hot-key-blocks=2  (report-only)")
	require.Contains(t, got, "Exit: 0")
}

func TestCLIFooter_PerfRatioUnavailable(t *testing.T) {
	r, out, _ := newReporter()
	// Hot keys reported but no execution stats wired (e.g. the fork-side
	// stats observer is not available): the ratio must read n/a, not 0.00.
	r.Footer(report.Summary{
		TotalBlocks:  3,
		OKCount:      3,
		HotKeyBlocks: 2,
	}, false)
	got := out.String()
	require.Contains(t, got, "Perf  max-exec-ratio=n/a  hot-key-blocks=2")
	require.NotContains(t, got, "0.00")
}

func TestCLIFooter_PerfLineAbsentWhenNoStats(t *testing.T) {
	r, out, _ := newReporter()
	r.Footer(report.Summary{TotalBlocks: 3, OKCount: 3}, false)
	require.NotContains(t, out.String(), "Perf ")
}

func TestJSONReporter_PerfFields(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewJSON(&buf)
	r.Header("corpus", 1, 1)
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       1,
		FixtureName: "evm-parallel",
		Verdict:     compare.Match,
		HotKeys: []compare.HotKeyStat{
			{Store: "acc", Key: "01aa", Conflicts: 3, Txs: []int{1, 2}},
		},
		ExecutionRatio: 2.5,
	})
	r.Footer(report.Summary{
		TotalBlocks:  1,
		OKCount:      1,
		HotKeyBlocks: 1,
		MaxExecRatio: 2.5,
	}, false)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))

	blk := doc["blocks"].([]any)[0].(map[string]any)
	require.Equal(t, float64(2.5), blk["execution_ratio"])
	hotKeys := blk["hot_keys"].([]any)
	require.Len(t, hotKeys, 1)
	hk := hotKeys[0].(map[string]any)
	require.Equal(t, "acc", hk["store"])
	require.Equal(t, "01aa", hk["key"])
	require.Equal(t, float64(3), hk["conflicts"])

	s := doc["summary"].(map[string]any)
	require.Equal(t, float64(1), s["hot_key_blocks"])
	require.Equal(t, float64(2.5), s["max_execution_ratio"])
	require.Equal(t, float64(0), s["exit_code"])
}

func TestMarkdownReporter_PerfSection(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("fixtures", 2, 1)
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       2,
		FixtureName: "evm-parallel",
		Verdict:     compare.Match,
		HotKeys: []compare.HotKeyStat{
			{Store: "acc", Key: "01aa", Conflicts: 5, Txs: []int{0, 1, 2}},
		},
		ExecutionRatio: 2.0,
	})
	r.Block(report.BlockOutcome{
		Index: 2, Total: 2, FixtureName: "clean", Verdict: compare.Match, ExecutionRatio: 1.0,
	})
	r.Footer(report.Summary{
		TotalBlocks:  2,
		OKCount:      2,
		HotKeyBlocks: 1,
		MaxExecRatio: 2.0,
	}, false)

	out := buf.String()
	require.Contains(t, out, "| Max Execution Ratio | 2.00 |")
	require.Contains(t, out, "## Hot Conflict Keys")
	require.Contains(t, out, "Max execution ratio: **2.00**")
	require.Contains(t, out, "### evm-parallel")
	require.Contains(t, out, "Execution ratio: **2.00**")
	require.Contains(t, out, "| Store | Key | Conflicts | Distinct Txs |")
	require.Contains(t, out, "| acc | `01aa` | 5 | 3 |")
	require.NotContains(t, out, "### clean", "blocks without hot keys stay out of the section")
}

func TestMarkdownReporter_PerfRatioUnavailable(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("fixtures", 1, 1)
	// Hot keys present but no execution stats wired: the section's ratio must
	// read n/a, matching the CLI, instead of a misleading 0.00.
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       1,
		FixtureName: "evm-parallel",
		Verdict:     compare.Match,
		HotKeys:     []compare.HotKeyStat{{Store: "acc", Key: "01aa", Conflicts: 2, Txs: []int{0, 1}}},
	})
	r.Footer(report.Summary{TotalBlocks: 1, OKCount: 1, HotKeyBlocks: 1}, false)

	out := buf.String()
	require.Contains(t, out, "Max execution ratio: **n/a**")
	require.NotContains(t, out, "0.00")
}

func TestMarkdownReporter_PerfSectionAbsentWhenClean(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewMarkdown(&buf)
	r.Header("fixtures", 1, 1)
	r.Block(report.BlockOutcome{Index: 1, Total: 1, FixtureName: "clean", Verdict: compare.Match})
	r.Footer(report.Summary{TotalBlocks: 1, OKCount: 1}, false)
	got := buf.String()
	require.NotContains(t, got, "Hot Conflict Keys")
	require.NotContains(t, got, "Max Execution Ratio")
}
