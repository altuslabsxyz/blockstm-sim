package detect

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectJSONReporter_SchemaVersion(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)
	r.Header("../cosmos-sdk")
	r.Footer(&ScanResult{Files: 10}, "../cosmos-sdk")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	require.Equal(t, float64(1), doc["schema_version"])
	require.Equal(t, "../cosmos-sdk", doc["sdk_path"])
}

func TestDetectJSONReporter_FindingFields(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)
	r.Header("../sdk")
	r.Finding(Finding{
		Category: CatTime,
		File:     "x/bank/keeper.go",
		Line:     42,
		FuncName: "SendCoins",
		Call:     "time.Now",
		Module:   "bank",
	})
	r.Footer(&ScanResult{Files: 1, Findings: []Finding{{Category: CatTime}}}, "../sdk")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	findings := doc["findings"].([]any)
	require.Len(t, findings, 1)
	f := findings[0].(map[string]any)
	require.Equal(t, "time", f["category"])
	require.Equal(t, "x/bank/keeper.go", f["file"])
	require.Equal(t, float64(42), f["line"])
	require.Equal(t, "SendCoins", f["func_name"])
	require.Equal(t, "time.Now", f["call"])
	require.Equal(t, "bank", f["module"])
}

func TestDetectJSONReporter_SummaryCategories(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)
	r.Header("../sdk")
	r.Finding(Finding{Category: CatTime})
	r.Finding(Finding{Category: CatTime})
	r.Finding(Finding{Category: CatRand})
	r.Finding(Finding{Category: CatIO})
	r.Footer(&ScanResult{
		Files: 50,
		Findings: []Finding{
			{Category: CatTime}, {Category: CatTime},
			{Category: CatRand}, {Category: CatIO},
		},
	}, "../sdk")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	s := doc["summary"].(map[string]any)
	require.Equal(t, float64(4), s["total"])
	require.Equal(t, float64(2), s["time"])
	require.Equal(t, float64(1), s["rand"])
	require.Equal(t, float64(1), s["io"])
	require.Equal(t, float64(50), s["files_scanned"])
}

func TestDetectJSONReporter_EmptyFindings(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)
	r.Header("../sdk")
	r.Footer(&ScanResult{Files: 5}, "../sdk")

	var doc map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))
	findings := doc["findings"].([]any)
	require.Len(t, findings, 0)
}
