package detect

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReportHeader(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf)
	r.Header("../cosmos-sdk")

	require.Equal(t, "Detect  sdk-path=../cosmos-sdk\n\n", buf.String())
}

func TestReportFinding(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf)
	r.Finding(Finding{
		Category: CatTime,
		File:     "x/bank/keeper/msg_server.go",
		Line:     42,
		FuncName: "SendCoins",
		Call:     "time.Now",
		Module:   "bank",
	})

	want := "[time] x/bank/keeper/msg_server.go:42  SendCoins\n       time.Now\n"
	require.Equal(t, want, buf.String())
}

func TestReportFooter(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf)
	r.Footer(&ScanResult{
		Findings: []Finding{
			{Category: CatTime},
			{Category: CatRand},
			{Category: CatIO},
		},
		Files: 142,
	}, "../cosmos-sdk")

	want := "\nSummary\n  3 findings / 1 time / 1 rand / 1 io\n  Scanned 142 files in ../cosmos-sdk\n"
	require.Equal(t, want, buf.String())
}

func TestReportFooter_Empty(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf)
	r.Footer(&ScanResult{Files: 50}, "../cosmos-sdk")

	want := "\nSummary\n  0 findings / 0 time / 0 rand / 0 io\n  Scanned 50 files in ../cosmos-sdk\n"
	require.Equal(t, want, buf.String())
}
