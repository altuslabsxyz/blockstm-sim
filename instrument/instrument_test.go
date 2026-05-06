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
	"github.com/cosmos/cosmos-sdk/baseapp/lifecycle"

	"github.com/altuslabsxyz/blockstm-sim/instrument"
)

var _ instrument.Instrumentable = (*mockApp)(nil)

type mockApp struct {
	observerSet bool
	observer    lifecycle.LifecycleObserver
	runnerUnset bool
}

func (m *mockApp) SetLifecycleObserver(obs lifecycle.LifecycleObserver) {
	m.observerSet = true
	m.observer = obs
}

func (m *mockApp) UnsetBlockSTMTxRunner() {
	m.runnerUnset = true
}

func TestInstrumentApp(t *testing.T) {
	tests := []struct {
		name         string
		opts         instrument.Options
		wantObsSet   bool
		wantRunUnset bool
	}{
		{
			name:         "zero_options",
			opts:         instrument.Options{},
			wantObsSet:   false,
			wantRunUnset: false,
		},
		{
			name:         "observer_only",
			opts:         instrument.Options{Observer: lifecycle.NoopLifecycleObserver{}},
			wantObsSet:   true,
			wantRunUnset: false,
		},
		{
			name:         "runner_sequential",
			opts:         instrument.Options{Runner: instrument.RunnerSequential},
			wantObsSet:   false,
			wantRunUnset: true,
		},
		{
			name:         "both_options",
			opts:         instrument.Options{Observer: lifecycle.NoopLifecycleObserver{}, Runner: instrument.RunnerSequential},
			wantObsSet:   true,
			wantRunUnset: true,
		},
		{
			name:         "runner_stm_explicit",
			opts:         instrument.Options{Runner: instrument.RunnerSTM},
			wantObsSet:   false,
			wantRunUnset: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := &mockApp{}
			instrument.InstrumentApp(app, tc.opts)
			require.Equal(t, tc.wantObsSet, app.observerSet, "observerSet")
			require.Equal(t, tc.wantRunUnset, app.runnerUnset, "runnerUnset")
		})
	}
}

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
