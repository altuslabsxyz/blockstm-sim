//go:build sdk_hooks && simharness_canary

package run_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/report"
	"github.com/altuslabsxyz/blockstm-sim/run"
)

// maxAttempts bounds the retry loop that accounts for the inherent
// non-determinism of the BlockSTM goroutine race in the C1 canary.
const maxAttempts = 20

func TestCanaryC1_DivergenceDetected(t *testing.T) {
	fixture, err := compare.LoadFixture("../corpus/fixtures", "canary-01-keeper-map.yaml")
	require.NoError(t, err)
	require.True(t, fixture.IsCanary(), "fixture must be kind=canary")

	var diverged bool
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		exec := run.NewFixtureExecutor()
		err := exec.Init(fixture.Genesis)
		require.NoError(t, err)

		result, err := exec.RunBlock(fixture.Blocks[0], 1)
		exec.Close()
		require.NoError(t, err)

		if result.Verdict == compare.Divergence {
			t.Logf("divergence detected on attempt %d/%d", attempt, maxAttempts)
			require.NotEmpty(t, result.Findings)
			var hasAppHash bool
			for _, f := range result.Findings {
				if f.Dimension == compare.DimAppHash {
					require.NotEqual(t, f.Oracle, f.Probe)
					hasAppHash = true
				}
			}
			require.True(t, hasAppHash, "divergence must include app_hash finding")
			diverged = true
			break
		}
		t.Logf("attempt %d/%d: match (no divergence yet)", attempt, maxAttempts)
	}

	require.True(t, diverged,
		"canary-01-keeper-map must produce at least one DIVERGENCE in %d attempts; "+
			"if this fails consistently, the in-memory map race is not triggering", maxAttempts)
}

func TestCanaryC1_HarnessE2E(t *testing.T) {
	fixtureContent, err := os.ReadFile("../corpus/fixtures/canary-01-keeper-map.yaml")
	require.NoError(t, err)

	var diverged bool
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, "canary-01-keeper-map.yaml"),
			fixtureContent, 0o644,
		))

		stores, err := compare.LoadCorpusStores(dir)
		require.NoError(t, err)

		exec := run.NewFixtureExecutor()
		out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
		rep := report.NewCLI(out, errOut)
		code := run.RunHarness(run.Config{CorpusDir: dir, Probes: 1}, exec, stores, rep, errOut)

		output := out.String()
		if strings.Contains(output, "DIVERGENCE canary-01-keeper-map") {
			t.Logf("harness E2E divergence on attempt %d", attempt)
			require.Equal(t, 0, code, "expected canary divergence gives exit 0")
			require.Contains(t, output, "1 canary expected")
			require.Contains(t, output, "0 canary missed")
			diverged = true
			break
		}
		t.Logf("attempt %d/%d: no divergence in harness output", attempt, maxAttempts)
	}

	require.True(t, diverged,
		"harness must report DIVERGENCE for canary-01-keeper-map within %d attempts", maxAttempts)
}

// TestCanaryF4_OutOfKVStoreMutationDetected verifies that the F4 runtime layer
// generates DimOutOfKVStore findings deterministically on each run.
// Unlike C1 (race-dependent), F4 detects snapshot diffs on every oracle execution.
func TestCanaryF4_OutOfKVStoreMutationDetected(t *testing.T) {
	fixture, err := compare.LoadFixture("../corpus/fixtures", "canary-01-keeper-map.yaml")
	require.NoError(t, err)

	// WithSTMOracle enables lifecycle callbacks (OnTxStart/OnTxEnd) which are
	// required for F4 out-of-KVStore mutation detection. The default sequential
	// runner does not fire those callbacks.
	exec := run.NewFixtureExecutor(run.WithSTMOracle(4))
	require.NoError(t, exec.Init(fixture.Genesis))
	defer exec.Close()

	result, err := exec.RunBlock(fixture.Blocks[0], 1)
	require.NoError(t, err)

	var f4Findings []compare.Finding
	for _, f := range result.Findings {
		if f.Dimension == compare.DimOutOfKVStore {
			f4Findings = append(f4Findings, f)
		}
	}
	require.NotEmpty(t, f4Findings,
		"F4 must detect out-of-KVStore mutation in canary-01-keeper-map on every run")
	require.Equal(t, 0, f4Findings[0].TxIndex,
		"MapSet (tx 0) mutates sharedMap and must be detected by F4")
	require.Contains(t, f4Findings[0].Oracle, "simcanary.sharedMap")
	require.Contains(t, f4Findings[0].Probe, "simcanary.sharedMap")
	require.NotEqual(t, f4Findings[0].Oracle, f4Findings[0].Probe,
		"before and after snapshots must differ")
}
