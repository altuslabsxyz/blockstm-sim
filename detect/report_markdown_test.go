package detect

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectMarkdownReporter_Title(t *testing.T) {
	var buf bytes.Buffer
	r := NewMarkdownReporter(&buf)
	r.Header("../stable-sdk")
	r.Footer(&ScanResult{Files: 10}, "../stable-sdk")

	out := buf.String()
	require.Contains(t, out, "# BlockSTM Sim — Detect Report")
	require.Contains(t, out, "**SDK Path:** ../stable-sdk")
}

func TestDetectMarkdownReporter_SummaryTable(t *testing.T) {
	var buf bytes.Buffer
	r := NewMarkdownReporter(&buf)
	r.Header("../sdk")
	r.Finding(Finding{Category: CatTime})
	r.Finding(Finding{Category: CatTime})
	r.Finding(Finding{Category: CatRand})
	r.Footer(&ScanResult{
		Files:    42,
		Findings: []Finding{{Category: CatTime}, {Category: CatTime}, {Category: CatRand}},
	}, "../sdk")

	out := buf.String()
	require.Contains(t, out, "## Summary")
	require.Contains(t, out, "| time | 2 |")
	require.Contains(t, out, "| rand | 1 |")
	require.Contains(t, out, "| io | 0 |")
	require.Contains(t, out, "**Total** | **3**")
	require.Contains(t, out, "Scanned 42 files.")
}

func TestDetectMarkdownReporter_GroupedByModule(t *testing.T) {
	var buf bytes.Buffer
	r := NewMarkdownReporter(&buf)
	r.Header("../sdk")
	r.Finding(Finding{
		Category: CatTime,
		File:     "x/bank/keeper.go",
		Line:     10,
		FuncName: "Foo",
		Call:     "time.Now",
		Module:   "bank",
	})
	r.Finding(Finding{
		Category: CatRand,
		File:     "x/staking/keeper.go",
		Line:     20,
		FuncName: "Bar",
		Call:     "rand.Int",
		Module:   "staking",
	})
	r.Footer(&ScanResult{
		Files:    5,
		Findings: []Finding{{Module: "bank"}, {Module: "staking"}},
	}, "../sdk")

	out := buf.String()
	require.Contains(t, out, "## Findings by Module")
	require.Contains(t, out, "### bank")
	require.Contains(t, out, "### staking")
	require.Contains(t, out, "`x/bank/keeper.go`")
	require.Contains(t, out, "`time.Now`")
	require.Contains(t, out, "`x/staking/keeper.go`")
	require.Contains(t, out, "`rand.Int`")
}

func TestDetectMarkdownReporter_ModulesAreSorted(t *testing.T) {
	var buf bytes.Buffer
	r := NewMarkdownReporter(&buf)
	r.Header("../sdk")
	r.Finding(Finding{Module: "staking", Category: CatTime, File: "a", FuncName: "f", Call: "c"})
	r.Finding(Finding{Module: "bank", Category: CatRand, File: "b", FuncName: "g", Call: "d"})
	r.Footer(&ScanResult{Files: 2, Findings: []Finding{{}, {}}}, "../sdk")

	out := buf.String()
	bankIdx := strings.Index(out, "### bank")
	stakingIdx := strings.Index(out, "### staking")
	require.Greater(t, bankIdx, -1)
	require.Greater(t, stakingIdx, -1)
	require.Less(t, bankIdx, stakingIdx, "bank should appear before staking (alphabetical)")
}

func TestDetectMarkdownReporter_ReproduceCommand(t *testing.T) {
	var buf bytes.Buffer
	r := NewMarkdownReporter(&buf)
	r.Header("../my-sdk")
	r.Footer(&ScanResult{}, "../my-sdk")

	out := buf.String()
	require.Contains(t, out, "## Reproduce")
	require.Contains(t, out, "blockstm-sim detect --sdk-path ../my-sdk")
}

func TestDetectMarkdownReporter_NoFindingsSection_WhenEmpty(t *testing.T) {
	var buf bytes.Buffer
	r := NewMarkdownReporter(&buf)
	r.Header("../sdk")
	r.Footer(&ScanResult{Files: 5}, "../sdk")

	out := buf.String()
	require.NotContains(t, out, "## Findings by Module")
}
