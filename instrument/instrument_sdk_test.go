//go:build sdk_hooks

package instrument_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"

	"github.com/altuslabsxyz/blockstm-sim/instrument"
)

func newTestBaseApp(t *testing.T) *baseapp.BaseApp {
	t.Helper()
	app := baseapp.NewBaseApp(t.Name(), log.NewNopLogger(), dbm.NewMemDB(), nil)
	app.MountStores(storetypes.NewKVStoreKey("test"))
	require.NoError(t, app.LoadLatestVersion())
	return app
}

func TestInstrumentApp_OracleProbeAppHash(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	oracleRes, err := oracle.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 1})
	require.NoError(t, err)
	probeRes, err := probe.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 1})
	require.NoError(t, err)

	require.NotEmpty(t, oracleRes.AppHash, "oracle AppHash must not be empty")
	require.Equal(t, oracleRes.AppHash, probeRes.AppHash,
		"oracle (sequential) and probe (STM) must produce identical app hash on empty block")
}
