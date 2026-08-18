package run_test

import (
	"bytes"
	"context"
	"errors"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/report"
	"github.com/altuslabsxyz/blockstm-sim/run"
)

type mockExecutor struct {
	results []*compare.Result
	idx     int
}

func (m *mockExecutor) Init(compare.GenesisSpec) error { return nil }
func (m *mockExecutor) Close()                         {}
func (m *mockExecutor) RunBlock(block compare.BlockSpec, _ int64) (*compare.Result, error) {
	var r *compare.Result
	if m.idx >= len(m.results) {
		r = &compare.Result{Verdict: compare.Match}
	} else {
		r = m.results[m.idx]
		m.idx++
	}
	// populate MsgKeys from block spec, mirroring what FixtureExecutor does
	r.MsgKeys = make([]string, len(block.Txs))
	for i, tx := range block.Txs {
		r.MsgKeys[i] = tx.Msg
	}
	return r, nil
}

func writeFixture(t *testing.T, dir, name, kind string, blocks int) {
	t.Helper()
	kindLine := ""
	if kind != "" {
		kindLine = "kind: " + kind + "\n"
	}
	blockYAML := ""
	for range blocks {
		blockYAML += `
  - txs:
      - signer: sender
        msg: bank-send
        to: receiver
        amount: "100stake"
        gas: 200000
`
	}
	content := "name: " + name + "\n" +
		"description: test\n" +
		kindLine +
		"genesis:\n  accounts:\n    sender:\n      balance: \"1000000stake\"\n    receiver:\n      balance: \"500000stake\"\n" +
		"blocks:" + blockYAML
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(content), 0o644))
}

func loadStores(t *testing.T, dir string) []compare.CorpusStore {
	t.Helper()
	stores, err := compare.LoadCorpusStores(dir)
	require.NoError(t, err)
	return stores
}

func TestHarness_AllOK(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "01-test", "", 2)

	exec := &mockExecutor{results: []*compare.Result{
		{Verdict: compare.Match},
		{Verdict: compare.Match},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	rep := report.NewCLI(out, errOut)
	code := run.RunHarness(run.Config{CorpusDir: dir, Probes: 1}, exec, loadStores(t, dir), rep, errOut)

	require.Equal(t, 0, code)
	require.Contains(t, out.String(), "2 blocks run / 2 ok / 0 divergence")
	require.Contains(t, out.String(), "Exit: 0")
}

func TestHarness_DivergenceNoFlag(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "01-test", "", 1)

	exec := &mockExecutor{results: []*compare.Result{
		{Verdict: compare.Divergence, Findings: []compare.Finding{
			{Dimension: compare.DimAppHash, Oracle: "aabb", Probe: "ccdd"},
		}},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	rep := report.NewCLI(out, errOut)
	code := run.RunHarness(run.Config{CorpusDir: dir, Probes: 1}, exec, loadStores(t, dir), rep, errOut)

	require.Equal(t, 0, code)
	require.Contains(t, out.String(), "DIVERGENCE 01-test")
	require.Contains(t, out.String(), "1 divergence")
}

func TestHarness_DivergenceWithFlag(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "01-test", "", 1)

	exec := &mockExecutor{results: []*compare.Result{
		{Verdict: compare.Divergence},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	rep := report.NewCLI(out, errOut)
	code := run.RunHarness(run.Config{CorpusDir: dir, Probes: 1, FailOnDivergence: true}, exec, loadStores(t, dir), rep, errOut)

	require.Equal(t, 1, code)
	require.Contains(t, out.String(), "Exit: 1")
}

func TestHarness_CanaryExpected(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "canary-01", "canary", 1)

	exec := &mockExecutor{results: []*compare.Result{
		{Verdict: compare.Divergence},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	rep := report.NewCLI(out, errOut)
	code := run.RunHarness(run.Config{CorpusDir: dir, Probes: 1}, exec, loadStores(t, dir), rep, errOut)

	require.Equal(t, 0, code)
	require.Contains(t, out.String(), "1 canary expected")
	require.Contains(t, out.String(), "0 canary missed")
}

func TestHarness_CanaryMissed(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "canary-01", "canary", 1)

	exec := &mockExecutor{results: []*compare.Result{
		{Verdict: compare.Match},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	rep := report.NewCLI(out, errOut)
	code := run.RunHarness(run.Config{CorpusDir: dir, Probes: 1}, exec, loadStores(t, dir), rep, errOut)

	require.Equal(t, 1, code)
	require.Contains(t, out.String(), "CANARY MISSED canary-01")
	require.Contains(t, out.String(), "1 canary missed")
}

func TestHarness_MixedScenario(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "01-normal", "", 1)
	writeFixture(t, dir, "canary-01", "canary", 1)

	exec := &mockExecutor{results: []*compare.Result{
		{Verdict: compare.Match},
		{Verdict: compare.Divergence},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	rep := report.NewCLI(out, errOut)
	code := run.RunHarness(run.Config{CorpusDir: dir, Probes: 1}, exec, loadStores(t, dir), rep, errOut)

	require.Equal(t, 0, code)
	require.Contains(t, out.String(), "ok 01-normal")
	require.Contains(t, out.String(), "DIVERGENCE canary-01")
	require.Contains(t, out.String(), "1 ok / 0 divergence (1 canary expected) / 0 canary missed")
}

func TestHarness_EmptyStores(t *testing.T) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	rep := report.NewCLI(out, errOut)
	code := run.RunHarness(run.Config{CorpusDir: "/nonexistent"}, nil, nil, rep, errOut)

	require.Equal(t, 0, code)
}

func TestHarness_CoverageInFooter(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "01-test", "", 1)

	exec := &mockExecutor{results: []*compare.Result{
		{Verdict: compare.Match},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	run.RunHarness(run.Config{CorpusDir: dir, Probes: 1}, exec, loadStores(t, dir), report.NewCLI(out, errOut), errOut)

	got := out.String()
	// "bank-send" is registered by executor.init(); the fixture uses bank-send
	require.Contains(t, got, "Coverage")
	require.Contains(t, got, "bank")
	require.Contains(t, got, "MsgSend")
}

// mockSnapshotStore is a CorpusStore that mimics a SnapshotCorpus:
// SnapshotDir() returns a non-empty path and Iter yields blocks with Height set.
type mockSnapshotStore struct {
	dir    string
	meta   compare.RangeMeta
	blocks []compare.Block
	closed bool
}

func (s *mockSnapshotStore) Iter(_ context.Context) iter.Seq2[compare.Block, error] {
	return func(yield func(compare.Block, error) bool) {
		for _, b := range s.blocks {
			if !yield(b, nil) {
				return
			}
		}
	}
}
func (s *mockSnapshotStore) SnapshotDir() string          { return s.dir }
func (s *mockSnapshotStore) Meta() compare.RangeMeta      { return s.meta }
func (s *mockSnapshotStore) BondDenom() string            { return "uatom" }
func (s *mockSnapshotStore) Name() string                 { return "snap-test" }
func (s *mockSnapshotStore) IsCanary() bool               { return false }
func (s *mockSnapshotStore) Genesis() compare.GenesisSpec { return compare.GenesisSpec{} }
func (s *mockSnapshotStore) BlockCount() int              { return len(s.blocks) }
func (s *mockSnapshotStore) Close() error                 { s.closed = true; return nil }

// mockStateExecutor wraps mockExecutor and also implements StateInitializer.
type mockStateExecutor struct {
	mockExecutor
	initFromStateCalled bool
	initFromStateDir    string
	initFromStateMeta   compare.RangeMeta
	initFromStateErr    error
}

func (e *mockStateExecutor) InitFromState(dir string, meta compare.RangeMeta) error {
	e.initFromStateCalled = true
	e.initFromStateDir = dir
	e.initFromStateMeta = meta
	return e.initFromStateErr
}

func TestHarness_StateInitializer_Supported(t *testing.T) {
	meta := compare.RangeMeta{ChainID: "x", Start: 1000, End: 1000, BondDenom: "uatom"}
	store := &mockSnapshotStore{
		dir:  "/tmp/snap",
		meta: meta,
		blocks: []compare.Block{
			{Height: 1000, RawTxs: [][]byte{[]byte("tx")}},
		},
	}
	exec := &mockStateExecutor{
		mockExecutor: mockExecutor{results: []*compare.Result{{Verdict: compare.Match}}},
	}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	code := run.RunHarness(run.Config{CorpusDir: "snap", Probes: 1}, exec, []compare.CorpusStore{store}, report.NewCLI(out, errOut), errOut)

	require.Equal(t, 0, code)
	require.True(t, exec.initFromStateCalled, "InitFromState should have been called")
	require.Equal(t, "/tmp/snap", exec.initFromStateDir)
	require.Equal(t, meta, exec.initFromStateMeta)
	require.True(t, store.closed, "store.Close() should have been called after iteration")
	require.Empty(t, errOut.String())
}

func TestHarness_StateInitializer_NotSupported(t *testing.T) {
	store := &mockSnapshotStore{
		dir:    "/tmp/snap",
		blocks: []compare.Block{{Height: 1000}},
	}
	// mockExecutor does NOT implement StateInitializer.
	exec := &mockExecutor{}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	run.RunHarness(run.Config{CorpusDir: "snap", Probes: 1}, exec, []compare.CorpusStore{store}, report.NewCLI(out, errOut), errOut)

	require.Contains(t, errOut.String(), "executor does not support state-based init")
	require.True(t, store.closed, "store.Close() should be called even when executor lacks StateInitializer")
}

func TestHarness_StateInitializer_InitError(t *testing.T) {
	store := &mockSnapshotStore{
		dir:    "/tmp/snap",
		blocks: []compare.Block{{Height: 1000}},
	}
	exec := &mockStateExecutor{initFromStateErr: errors.New("state load failed")}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	run.RunHarness(run.Config{CorpusDir: "snap", Probes: 1}, exec, []compare.CorpusStore{store}, report.NewCLI(out, errOut), errOut)

	require.Contains(t, errOut.String(), "state load failed")
	require.True(t, store.closed, "store.Close() should be called even when InitFromState fails")
}

func TestHarness_PerfReporting(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "01-test", "", 2)

	exec := &mockExecutor{results: []*compare.Result{
		{
			Verdict: compare.Match,
			HotKeys: []compare.HotKeyStat{
				// 4 distinct txs → passes HotKeyMinTxs=2
				{Store: "acc", Key: "01aa", Conflicts: 9, Txs: []int{0, 1, 2, 3}},
				// 1 distinct tx → filtered out
				{Store: "bank", Key: "02bb", Conflicts: 1, Txs: []int{2}},
			},
			ExecutionRatio: 3.0,
		},
		{Verdict: compare.Match, ExecutionRatio: 1.0},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	cfg := run.Config{CorpusDir: dir, Probes: 1, HotKeyMinTxs: 2}
	code := run.RunHarness(cfg, exec, loadStores(t, dir), report.NewCLI(out, errOut), errOut)

	// Perf findings are report-only: exit code stays 0.
	require.Equal(t, 0, code)
	got := out.String()
	// The execution ratio is always reported when available — no threshold.
	require.Contains(t, got, "exec-ratio 3.00")
	require.Contains(t, got, "exec-ratio 1.00")
	require.Contains(t, got, "hot key acc/0x01aa")
	require.NotContains(t, got, "02bb", "below-threshold key must be filtered")
	require.Contains(t, got, "Perf  max-exec-ratio=3.00  hot-key-blocks=1")
}

// mockPerfExecutor wraps mockExecutor and records SetConflictDetection calls.
type mockPerfExecutor struct {
	mockExecutor
	conflictDetectionSet []bool
}

func (e *mockPerfExecutor) SetConflictDetection(enabled bool) {
	e.conflictDetectionSet = append(e.conflictDetectionSet, enabled)
}

func TestHarness_ConflictDetectionEnabled(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "01-test", "", 1)

	// HotKeyMinTxs > 0 → detection enabled on the executor.
	exec := &mockPerfExecutor{}
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	run.RunHarness(run.Config{CorpusDir: dir, Probes: 1, HotKeyMinTxs: 2}, exec, loadStores(t, dir), report.NewCLI(out, errOut), errOut)

	require.Equal(t, []bool{true}, exec.conflictDetectionSet)
}

func TestHarness_ConflictDetectionDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "01-test", "", 1)

	// HotKeyMinTxs <= 0 → detection disabled, so no observer is installed.
	exec := &mockPerfExecutor{}
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	run.RunHarness(run.Config{CorpusDir: dir, Probes: 1}, exec, loadStores(t, dir), report.NewCLI(out, errOut), errOut)

	require.Equal(t, []bool{false}, exec.conflictDetectionSet)
}

func TestHarness_HotKeyReportingDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "01-test", "", 1)

	exec := &mockExecutor{results: []*compare.Result{
		{
			Verdict:        compare.Match,
			HotKeys:        []compare.HotKeyStat{{Store: "acc", Key: "01aa", Conflicts: 9, Txs: []int{0, 1}}},
			ExecutionRatio: 9.0,
		},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	// HotKeyMinTxs = 0 disables hot-key reporting; the ratio is still reported.
	cfg := run.Config{CorpusDir: dir, Probes: 1}
	code := run.RunHarness(cfg, exec, loadStores(t, dir), report.NewCLI(out, errOut), errOut)

	require.Equal(t, 0, code)
	require.NotContains(t, out.String(), "hot key")
	require.Contains(t, out.String(), "exec-ratio 9.00")
}

func TestHarness_StatePatternInFooter(t *testing.T) {
	dir := t.TempDir()
	// Two blocks, each with one bank-send tx; distinct write sets → 2 patterns.
	writeFixture(t, dir, "01-test", "", 2)

	exec := &mockExecutor{results: []*compare.Result{
		{Verdict: compare.Match, TxWriteSets: [][]string{{"bank/aabb"}}},
		{Verdict: compare.Match, TxWriteSets: [][]string{{"bank/ccdd"}}},
	}}

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	run.RunHarness(run.Config{CorpusDir: dir, Probes: 1}, exec, loadStores(t, dir), report.NewCLI(out, errOut), errOut)

	require.Contains(t, out.String(), "2 state patterns")
}
