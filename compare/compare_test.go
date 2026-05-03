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
