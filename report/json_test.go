package report_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/report"
)

func TestJSONReporter_SchemaVersion(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewJSON(&buf)
	r.Header("fixtures", 1, 1)
	r.Footer(report.Summary{TotalBlocks: 1, OKCount: 1}, false)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	require.Equal(t, float64(1), doc["schema_version"])
	require.Equal(t, "fixtures", doc["corpus"])
	require.Equal(t, float64(1), doc["probes"])
}

func TestJSONReporter_MatchBlock(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewJSON(&buf)
	r.Header("corpus", 1, 2)
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       1,
		FixtureName: "01-bank-send",
		IsCanary:    false,
		Verdict:     compare.Match,
	})
	r.Footer(report.Summary{TotalBlocks: 1, OKCount: 1}, false)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	blocks := doc["blocks"].([]any)
	require.Len(t, blocks, 1)
	blk := blocks[0].(map[string]any)
	require.Equal(t, "01-bank-send", blk["fixture"])
	require.Equal(t, "MATCH", blk["verdict"])
	require.Equal(t, false, blk["is_canary"])
	require.Nil(t, blk["findings"])
}

func TestJSONReporter_DivergenceFindings(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewJSON(&buf)
	r.Header("corpus", 1, 1)
	r.Block(report.BlockOutcome{
		Index:       1,
		Total:       1,
		FixtureName: "canary-01",
		IsCanary:    true,
		Verdict:     compare.Divergence,
		Findings: []compare.Finding{
			{
				ID:         "abc123def456",
				Height:     1,
				TxIndex:    -1,
				ProbeIndex: 0,
				Dimension:  compare.DimAppHash,
				Oracle:     "aaa",
				Probe:      "bbb",
			},
		},
	})
	r.Footer(report.Summary{TotalBlocks: 1, CanaryExpected: 1}, false)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	blocks := doc["blocks"].([]any)
	blk := blocks[0].(map[string]any)
	require.Equal(t, "DIVERGENCE", blk["verdict"])
	require.Equal(t, true, blk["is_canary"])
	findings := blk["findings"].([]any)
	require.Len(t, findings, 1)
	f := findings[0].(map[string]any)
	require.Equal(t, "abc123def456", f["id"])
	require.Equal(t, float64(1), f["height"])
	require.Equal(t, float64(-1), f["tx_index"])
	require.Equal(t, float64(0), f["probe_index"])
	require.Equal(t, "app_hash", f["dimension"])
	require.Equal(t, "aaa", f["oracle"])
	require.Equal(t, "bbb", f["probe"])
}

func TestJSONReporter_SummaryFields(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewJSON(&buf)
	r.Header("fixtures", 5, 1)
	r.Footer(report.Summary{
		TotalBlocks:     5,
		OKCount:         3,
		DivergenceCount: 1,
		CanaryExpected:  1,
		CanaryMissed:    0,
	}, false)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	s := doc["summary"].(map[string]any)
	require.Equal(t, float64(5), s["total_blocks"])
	require.Equal(t, float64(3), s["ok"])
	require.Equal(t, float64(1), s["divergences"])
	require.Equal(t, float64(1), s["canary_expected"])
	require.Equal(t, float64(0), s["canary_missed"])
	require.Equal(t, float64(0), s["exit_code"])
}

func TestJSONReporter_ExitCode(t *testing.T) {
	var buf bytes.Buffer
	r := report.NewJSON(&buf)
	r.Header("fixtures", 5, 1)
	r.Footer(report.Summary{TotalBlocks: 5, DivergenceCount: 1}, true)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	s := doc["summary"].(map[string]any)
	require.Equal(t, float64(1), s["exit_code"])
}

func TestJSONReporter_Errors(t *testing.T) {
	r := report.NewJSON(&bytes.Buffer{})
	require.Equal(t, 0, r.Errors())
}
