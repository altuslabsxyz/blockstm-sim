package instrument_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/instrument"
)

var _ instrument.Instrumentable = (*mockApp)(nil)

type mockApp struct {
	observerSet bool
	observer    compare.LifecycleObserver
	runnerUnset bool
}

func (m *mockApp) SetLifecycleObserver(obs compare.LifecycleObserver) {
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
			opts:         instrument.Options{Observer: compare.NoopLifecycleObserver{}},
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
			opts:         instrument.Options{Observer: compare.NoopLifecycleObserver{}, Runner: instrument.RunnerSequential},
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
