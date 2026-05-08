//go:build sdk_hooks

package compare_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/instrument"
)

// ---------------------------------------------------------------------------
// Test 1: MATCH on empty block (bare BaseApp, no modules)
// ---------------------------------------------------------------------------

func newTestBaseApp(t *testing.T) *baseapp.BaseApp {
	t.Helper()
	app := baseapp.NewBaseApp(t.Name(), log.NewNopLogger(), dbm.NewMemDB(), nil)
	app.MountStores(storetypes.NewKVStoreKey("test"))
	require.NoError(t, app.LoadLatestVersion())
	return app
}

func TestRun_Match_EmptyBlock(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle: oracle,
		Probe:  probe,
		Block:  &abci.RequestFinalizeBlock{Height: 1},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Match, result.Verdict)
	require.Equal(t, int64(1), result.Height)
	require.Empty(t, result.Findings)
}

// ---------------------------------------------------------------------------
// Test 2: MATCH on real MsgSend (full app with bank module)
// ---------------------------------------------------------------------------

func TestRun_Match_BankSend(t *testing.T) {
	fix, err := compare.LoadFixture("testdata", "01-single-bank-send.yaml")
	require.NoError(t, err)
	pair := newAppPair(t, fix.Genesis.Accounts)

	txBytes := buildBankSendTx(t, pair, fix.Blocks[0].Txs[0])

	result, err := compare.Run(compare.Input{
		Oracle: pair.Oracle,
		Probe:  pair.Probe,
		Block: &abci.RequestFinalizeBlock{
			Height: 1,
			Txs:    [][]byte{txBytes},
		},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Match, result.Verdict,
		"oracle (sequential) and probe (STM) must agree on app hash after MsgSend")
	require.Empty(t, result.Findings)
}

// ---------------------------------------------------------------------------
// Test 3: DIVERGENCE detected when probe hash is corrupted
// ---------------------------------------------------------------------------

func TestRun_Divergence(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle: oracle,
		Probe:  &divergentFinalizer{probe},
		Block:  &abci.RequestFinalizeBlock{Height: 1},
	})
	require.NoError(t, err)

	require.Equal(t, compare.Divergence, result.Verdict)
	require.Equal(t, int64(1), result.Height)
	require.Len(t, result.Findings, 1)

	f := result.Findings[0]
	require.Equal(t, compare.DimAppHash, f.Dimension)
	require.Equal(t, -1, f.TxIndex)
	require.Equal(t, int64(1), f.Height)
	require.NotEqual(t, f.Oracle, f.Probe)

	expectedID := compare.FindingID(1, compare.DimAppHash, -1, 0)
	require.Equal(t, expectedID, f.ID)
}

// ---------------------------------------------------------------------------
// Test 4: Finding ID is deterministic
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Test 5: DIVERGENCE on per-tx error code mismatch
// ---------------------------------------------------------------------------

func TestRun_ErrorCodeMismatch(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle: &errorCodeFinalizer{oracle, []uint32{0, 5}},
		Probe:  &errorCodeFinalizer{probe, []uint32{0, 7}},
		Block:  &abci.RequestFinalizeBlock{Height: 1},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Divergence, result.Verdict)

	var errorFindings []compare.Finding
	for _, f := range result.Findings {
		if f.Dimension == compare.DimErrorCode {
			errorFindings = append(errorFindings, f)
		}
	}
	require.Len(t, errorFindings, 1, "only tx 1 has different codes")
	require.Equal(t, 1, errorFindings[0].TxIndex)
	require.Equal(t, "5", errorFindings[0].Oracle)
	require.Equal(t, "7", errorFindings[0].Probe)
}

// ---------------------------------------------------------------------------
// Test 6: MATCH when app hash matches and error codes match
// ---------------------------------------------------------------------------

func TestRun_AppHashMatch_ErrorCodesMatch(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle: &errorCodeFinalizer{oracle, []uint32{0, 0}},
		Probe:  &errorCodeFinalizer{probe, []uint32{0, 0}},
		Block:  &abci.RequestFinalizeBlock{Height: 1},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Match, result.Verdict)
	require.Empty(t, result.Findings)
}

// ---------------------------------------------------------------------------
// Test 7: DIVERGENCE when app hash matches but error codes differ
// ---------------------------------------------------------------------------

func TestRun_AppHashMatch_ErrorCodeDiverges(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle: &errorCodeFinalizer{oracle, []uint32{0}},
		Probe:  &errorCodeFinalizer{probe, []uint32{3}},
		Block:  &abci.RequestFinalizeBlock{Height: 1},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Divergence, result.Verdict,
		"error code mismatch must produce DIVERGENCE even when AppHash matches")

	require.Len(t, result.Findings, 1)
	f := result.Findings[0]
	require.Equal(t, compare.DimErrorCode, f.Dimension)
	require.Equal(t, 0, f.TxIndex)
}

// ---------------------------------------------------------------------------
// Test 8: Write set mismatch
// ---------------------------------------------------------------------------

func TestRun_WriteSetMismatch(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	oracleWS := &mockWriteSetProvider{sets: map[int][]string{
		0: {"bank/01", "bank/02"},
		1: {"simcanary/ab"},
	}}
	probeWS := &mockWriteSetProvider{sets: map[int][]string{
		0: {"bank/01", "bank/02"},
		1: {"simcanary/ab", "simcanary/cd"},
	}}

	result, err := compare.Run(compare.Input{
		Oracle:          &errorCodeFinalizer{oracle, []uint32{0, 0}},
		Probe:           &errorCodeFinalizer{probe, []uint32{0, 0}},
		Block:           &abci.RequestFinalizeBlock{Height: 1},
		OracleWriteSets: oracleWS,
		ProbeWriteSets:  probeWS,
	})
	require.NoError(t, err)
	require.Equal(t, compare.Divergence, result.Verdict)

	var wsFindings []compare.Finding
	for _, f := range result.Findings {
		if f.Dimension == compare.DimWriteSet {
			wsFindings = append(wsFindings, f)
		}
	}
	require.Len(t, wsFindings, 1, "only tx 1 has different write sets")
	require.Equal(t, 1, wsFindings[0].TxIndex)
}

// ---------------------------------------------------------------------------
// Test 9: Write set match produces no finding
// ---------------------------------------------------------------------------

func TestRun_WriteSetMatch(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	ws := &mockWriteSetProvider{sets: map[int][]string{
		0: {"bank/01"},
	}}

	result, err := compare.Run(compare.Input{
		Oracle:          &errorCodeFinalizer{oracle, []uint32{0}},
		Probe:           &errorCodeFinalizer{probe, []uint32{0}},
		Block:           &abci.RequestFinalizeBlock{Height: 1},
		OracleWriteSets: ws,
		ProbeWriteSets:  ws,
	})
	require.NoError(t, err)
	require.Equal(t, compare.Match, result.Verdict)
	require.Empty(t, result.Findings)
}

// ---------------------------------------------------------------------------
// Test 10: Finding ID is deterministic
// ---------------------------------------------------------------------------

func TestFindingID_Deterministic(t *testing.T) {
	id1 := compare.FindingID(1, compare.DimAppHash, -1, 0)
	id2 := compare.FindingID(1, compare.DimAppHash, -1, 0)
	require.Equal(t, id1, id2, "same inputs must produce same ID")
	require.Len(t, id1, 12, "finding ID must be 12 hex chars")

	raw := fmt.Sprintf("%d|%s|%d|%d", 1, compare.DimAppHash, -1, 0)
	h := sha256.Sum256([]byte(raw))
	expected := hex.EncodeToString(h[:6])
	require.Equal(t, expected, id1, "ID must match sha256 spec")

	id3 := compare.FindingID(2, compare.DimAppHash, -1, 0)
	require.NotEqual(t, id1, id3, "different inputs must produce different IDs")

	id4 := compare.FindingID(1, compare.DimAppHash, 0, 0)
	require.NotEqual(t, id1, id4, "different txIndex must produce different IDs")
}

// ---------------------------------------------------------------------------
// Test 11: DimOutOfKVStore finding when oracle detects out-of-KVStore mutation
// ---------------------------------------------------------------------------

func TestRun_OutOfKVStoreMutation_Detected(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	oracleMuts := &mockMutationProvider{muts: map[int][]compare.MutationRecord{
		0: {{Tracker: "simcanary.sharedMap", Before: []byte(""), After: []byte("k=42")}},
	}}

	result, err := compare.Run(compare.Input{
		Oracle:          &errorCodeFinalizer{oracle, []uint32{0}},
		Probe:           &errorCodeFinalizer{probe, []uint32{0}},
		Block:           &abci.RequestFinalizeBlock{Height: 1},
		OracleMutations: oracleMuts,
	})
	require.NoError(t, err)
	require.Equal(t, compare.Divergence, result.Verdict)

	var mutFindings []compare.Finding
	for _, f := range result.Findings {
		if f.Dimension == compare.DimOutOfKVStore {
			mutFindings = append(mutFindings, f)
		}
	}
	require.Len(t, mutFindings, 1)
	require.Equal(t, 0, mutFindings[0].TxIndex)
	require.Contains(t, mutFindings[0].Oracle, "simcanary.sharedMap")
	require.Contains(t, mutFindings[0].Probe, "simcanary.sharedMap")
}

// ---------------------------------------------------------------------------
// Test 12: No DimOutOfKVStore finding when no mutations detected
// ---------------------------------------------------------------------------

func TestRun_OutOfKVStoreMutation_NoMutation_NoFinding(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	oracleMuts := &mockMutationProvider{muts: map[int][]compare.MutationRecord{}}

	result, err := compare.Run(compare.Input{
		Oracle:          oracle,
		Probe:           probe,
		Block:           &abci.RequestFinalizeBlock{Height: 1},
		OracleMutations: oracleMuts,
	})
	require.NoError(t, err)
	require.Equal(t, compare.Match, result.Verdict)
	require.Empty(t, result.Findings)
}

// ---------------------------------------------------------------------------
// Test: Gas mismatch
// ---------------------------------------------------------------------------

func TestRun_GasMismatch(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle: &gasFinalizer{oracle, []int64{1000, 2000}},
		Probe:  &gasFinalizer{probe, []int64{1000, 9999}},
		Block:  &abci.RequestFinalizeBlock{Height: 1},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Divergence, result.Verdict)

	var gasFindings []compare.Finding
	for _, f := range result.Findings {
		if f.Dimension == compare.DimGas {
			gasFindings = append(gasFindings, f)
		}
	}
	require.Len(t, gasFindings, 1, "only tx 1 has different gas")
	require.Equal(t, 1, gasFindings[0].TxIndex)
	require.Equal(t, "2000", gasFindings[0].Oracle)
	require.Equal(t, "9999", gasFindings[0].Probe)
}

func TestRun_GasMatch(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle: &gasFinalizer{oracle, []int64{1000}},
		Probe:  &gasFinalizer{probe, []int64{1000}},
		Block:  &abci.RequestFinalizeBlock{Height: 1},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Match, result.Verdict)
	require.Empty(t, result.Findings)
}

// ---------------------------------------------------------------------------
// Test: Events mismatch
// ---------------------------------------------------------------------------

func TestRun_EventsMismatch(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	oracleEvents := [][]abci.Event{
		{{Type: "transfer", Attributes: []abci.EventAttribute{{Key: "amount", Value: "100"}}}},
	}
	probeEvents := [][]abci.Event{
		{{Type: "transfer", Attributes: []abci.EventAttribute{{Key: "amount", Value: "999"}}}},
	}

	result, err := compare.Run(compare.Input{
		Oracle: &eventsFinalizer{oracle, oracleEvents},
		Probe:  &eventsFinalizer{probe, probeEvents},
		Block:  &abci.RequestFinalizeBlock{Height: 1},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Divergence, result.Verdict)

	var evtFindings []compare.Finding
	for _, f := range result.Findings {
		if f.Dimension == compare.DimEvents {
			evtFindings = append(evtFindings, f)
		}
	}
	require.Len(t, evtFindings, 1)
	require.Equal(t, 0, evtFindings[0].TxIndex)
	require.Contains(t, evtFindings[0].Oracle, "amount=100")
	require.Contains(t, evtFindings[0].Probe, "amount=999")
}

func TestRun_EventsMatch(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	events := [][]abci.Event{
		{{Type: "transfer", Attributes: []abci.EventAttribute{{Key: "amount", Value: "100"}}}},
	}

	result, err := compare.Run(compare.Input{
		Oracle: &eventsFinalizer{oracle, events},
		Probe:  &eventsFinalizer{probe, events},
		Block:  &abci.RequestFinalizeBlock{Height: 1},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Match, result.Verdict)
	require.Empty(t, result.Findings)
}

func TestRun_BlockContextMutation_GeneratesFinding(t *testing.T) {
	tracker := compare.NewBlockContextTracker(map[string]string{"height": "10"})
	tracker.SetCurrentTx(0)
	tracker.ReadField("height")
	tracker.SetCurrentTx(1)
	tracker.WriteField("height", "999")

	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle:                oracle,
		Probe:                 probe,
		Block:                 &abci.RequestFinalizeBlock{Height: 1},
		BlockContextMutations: tracker,
	})
	require.NoError(t, err)
	require.Equal(t, compare.Divergence, result.Verdict)
	require.Len(t, result.Findings, 1)
	f := result.Findings[0]
	require.Equal(t, compare.DimBlockContext, f.Dimension)
	require.Equal(t, 1, f.TxIndex)
	require.Contains(t, f.Oracle, "height")
	require.Contains(t, f.Oracle, "readers=0")
	require.Contains(t, f.Probe, "after=999")
}
