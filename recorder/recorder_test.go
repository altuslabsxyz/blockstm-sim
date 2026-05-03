package recorder

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "recorder-test: ", 0)
}

func TestNDJSONSink_WriteHeaderAndBlocks(t *testing.T) {
	dir := t.TempDir()
	runID := "test-run-001"

	sink, err := New(dir, runID, testLogger())
	require.NoError(t, err)

	header := RunRecord{
		SchemaVersion: SchemaVersion,
		RunID:         runID,
		StartedAt:     time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		CorpusDir:     "corpus/fixtures/",
		SimVersion:    "dev",
	}
	require.NoError(t, sink.WriteHeader(header))

	require.NoError(t, sink.WriteBlock(BlockRecord{
		Height:      1,
		FixtureName: "01-single-bank-send",
		Verdict:     "MATCH",
	}))
	require.NoError(t, sink.WriteBlock(BlockRecord{
		Height:      2,
		FixtureName: "02-multi-send",
		Verdict:     "DIVERGENCE",
		Divergences: []DivergenceEntry{{
			FindingID:  "a1b2c3d4e5f6",
			TxIndex:    -1,
			ProbeIndex: 0,
			Dimension:  "app_hash",
			Oracle:     "abc123",
			Probe:      "def456",
		}},
	}))

	require.NoError(t, sink.Close())

	f, err := os.Open(filepath.Join(dir, runID+".ndjson"))
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	scanner := bufio.NewScanner(f)

	// Line 1: RunRecord
	require.True(t, scanner.Scan())
	var gotHeader RunRecord
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &gotHeader))
	require.Equal(t, SchemaVersion, gotHeader.SchemaVersion)
	require.Equal(t, runID, gotHeader.RunID)
	require.Equal(t, "corpus/fixtures/", gotHeader.CorpusDir)
	require.Equal(t, "dev", gotHeader.SimVersion)

	// Line 2: MATCH block
	require.True(t, scanner.Scan())
	var block1 BlockRecord
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &block1))
	require.Equal(t, int64(1), block1.Height)
	require.Equal(t, "01-single-bank-send", block1.FixtureName)
	require.Equal(t, "MATCH", block1.Verdict)
	require.Empty(t, block1.Divergences)

	// Line 3: DIVERGENCE block
	require.True(t, scanner.Scan())
	var block2 BlockRecord
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &block2))
	require.Equal(t, int64(2), block2.Height)
	require.Equal(t, "DIVERGENCE", block2.Verdict)
	require.Len(t, block2.Divergences, 1)
	require.Equal(t, "a1b2c3d4e5f6", block2.Divergences[0].FindingID)
	require.Equal(t, "abc123", block2.Divergences[0].Oracle)
	require.Equal(t, "def456", block2.Divergences[0].Probe)

	require.False(t, scanner.Scan(), "expected exactly 3 lines")
}

func TestNDJSONSink_MatchBlock_NoDivergences(t *testing.T) {
	dir := t.TempDir()
	sink, err := New(dir, "match-test", testLogger())
	require.NoError(t, err)

	require.NoError(t, sink.WriteHeader(RunRecord{SchemaVersion: SchemaVersion, RunID: "match-test"}))
	require.NoError(t, sink.WriteBlock(BlockRecord{
		Height:      1,
		FixtureName: "simple",
		Verdict:     "MATCH",
	}))
	require.NoError(t, sink.Close())

	lines := splitLines(t, filepath.Join(dir, "match-test.ndjson"))
	require.Len(t, lines, 2)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(lines[1], &raw))
	_, hasDivergences := raw["divergences"]
	require.False(t, hasDivergences, "MATCH block should omit divergences field")
}

func TestNDJSONSink_DirCreation(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "sub", "deep")

	sink, err := New(nested, "dir-test", testLogger())
	require.NoError(t, err)
	require.NoError(t, sink.Close())

	_, err = os.Stat(filepath.Join(nested, "dir-test.ndjson"))
	require.NoError(t, err)
}

func TestNDJSONSink_FileNaming(t *testing.T) {
	dir := t.TempDir()
	runID := "20260429-120000-a1b2"

	sink, err := New(dir, runID, testLogger())
	require.NoError(t, err)
	require.NoError(t, sink.Close())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, runID+".ndjson", entries[0].Name())
}

func TestNopSink(t *testing.T) {
	var s NopSink
	require.NoError(t, s.WriteHeader(RunRecord{}))
	require.NoError(t, s.WriteBlock(BlockRecord{}))
	require.NoError(t, s.Close())
}

func TestBlockRecordFromResult_Match(t *testing.T) {
	r := &compare.Result{
		Verdict: compare.Match,
		Height:  5,
	}
	br := BlockRecordFromResult(r, "test-fixture")
	require.Equal(t, int64(5), br.Height)
	require.Equal(t, "test-fixture", br.FixtureName)
	require.Equal(t, "MATCH", br.Verdict)
	require.Empty(t, br.Divergences)
}

func TestBlockRecordFromResult_Divergence(t *testing.T) {
	r := &compare.Result{
		Verdict: compare.Divergence,
		Height:  10,
		Findings: []compare.Finding{
			compare.NewFinding(10, compare.DimAppHash, -1, 0, "oracleHash", "probeHash"),
		},
	}
	br := BlockRecordFromResult(r, "divergent-fixture")
	require.Equal(t, int64(10), br.Height)
	require.Equal(t, "DIVERGENCE", br.Verdict)
	require.Len(t, br.Divergences, 1)

	d := br.Divergences[0]
	require.Equal(t, compare.FindingID(10, compare.DimAppHash, -1, 0), d.FindingID)
	require.Equal(t, -1, d.TxIndex)
	require.Equal(t, 0, d.ProbeIndex)
	require.Equal(t, "app_hash", d.Dimension)
	require.Equal(t, "oracleHash", d.Oracle)
	require.Equal(t, "probeHash", d.Probe)
}

func TestGenerateRunID_Format(t *testing.T) {
	id := GenerateRunID()
	matched, err := regexp.MatchString(`^\d{8}-\d{6}-[0-9a-f]{8}$`, id)
	require.NoError(t, err)
	require.True(t, matched, "run ID %q must match YYYYMMDD-HHmmss-XXXX format", id)
}

func TestGenerateRunID_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		id := GenerateRunID()
		_, dup := seen[id]
		require.False(t, dup, "duplicate run ID: %s", id)
		seen[id] = struct{}{}
	}
}

// splitLines reads a file and returns each line as raw bytes.
func splitLines(t *testing.T, path string) [][]byte {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()

	var lines [][]byte
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())
		lines = append(lines, line)
	}
	require.NoError(t, scanner.Err())
	return lines
}
