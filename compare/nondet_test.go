//go:build sdk_hooks

package compare_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/instrument"
)

// mockNonDetProvider is a minimal NonDetProvider for unit tests.
type mockNonDetProvider struct {
	calls []compare.NonDetCall
}

func (m *mockNonDetProvider) NonDetCalls() []compare.NonDetCall {
	out := m.calls
	m.calls = nil
	return out
}
func (m *mockNonDetProvider) SetCurrentTx(int) {}

func TestRun_NonDetProvider_EmitsFinding(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	provider := &mockNonDetProvider{
		calls: []compare.NonDetCall{
			{Category: "time", CallSite: "time.Now", TxIndex: 0},
		},
	}

	result, err := compare.Run(compare.Input{
		Oracle:         oracle,
		Probe:          probe,
		Block:          &abci.RequestFinalizeBlock{Height: 1},
		NonDetProvider: provider,
	})
	require.NoError(t, err)
	require.Equal(t, compare.Divergence, result.Verdict)

	var ndFindings []compare.Finding
	for _, f := range result.Findings {
		if f.Dimension == compare.DimNonDeterministic {
			ndFindings = append(ndFindings, f)
		}
	}
	require.Len(t, ndFindings, 1)
	require.Equal(t, 0, ndFindings[0].TxIndex)
	require.Contains(t, ndFindings[0].Oracle, "time.Now")
}

func TestRun_NonDetProvider_NilProvider_NoFinding(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle:         oracle,
		Probe:          probe,
		Block:          &abci.RequestFinalizeBlock{Height: 1},
		NonDetProvider: nil,
	})
	require.NoError(t, err)
	require.Equal(t, compare.Match, result.Verdict)
	require.Empty(t, result.Findings)
}

func TestRun_NonDetProvider_EmptyProvider_NoFinding(t *testing.T) {
	oracle := newTestBaseApp(t)
	probe := newTestBaseApp(t)

	instrument.InstrumentApp(oracle, instrument.Options{Runner: instrument.RunnerSequential})

	_, err := oracle.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)
	_, err = probe.InitChain(&abci.RequestInitChain{})
	require.NoError(t, err)

	result, err := compare.Run(compare.Input{
		Oracle:         oracle,
		Probe:          probe,
		Block:          &abci.RequestFinalizeBlock{Height: 1},
		NonDetProvider: &mockNonDetProvider{},
	})
	require.NoError(t, err)
	require.Equal(t, compare.Match, result.Verdict)
	require.Empty(t, result.Findings)
}
