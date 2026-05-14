//go:build sdk_hooks

package run

import (
	"testing"

	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// testPruneFactory returns an AppFactory suitable for PruneSnapshot calls in
// tests. Full round-trip pruning behaviour (bootstrap → prune → reload) is
// exercised by chain-side integration tests against real `blockstm-sim extract`
// snapshots: simtestutil's AtGenesis:false setup cannot reliably load a DB that
// was created in the same process, since several module IAVL trees only stamp
// proper version entries when driven by full consensus flow (proposer hash,
// next-vals hash, etc.) rather than synthetic empty FinalizeBlock calls.
func testPruneFactory(db dbm.DB, txCfgOut *client.TxConfig, _ ...any) (sdkhook.App, any, error) {
	valSet, err := simtestutil.CreateRandomValidatorSet()
	if err != nil {
		return nil, nil, err
	}
	baseCfg := simtestutil.StartupConfig{
		DB:           db,
		AtGenesis:    false,
		ValidatorSet: func() (*cmttypes.ValidatorSet, error) { return valSet, nil },
	}
	var raw any
	if txCfgOut != nil {
		raw, err = simtestutil.SetupWithConfiguration(buildAppConfig(), baseCfg, txCfgOut)
	} else {
		raw, err = simtestutil.SetupWithConfiguration(buildAppConfig(), baseCfg)
	}
	if err != nil {
		return nil, nil, err
	}
	return sdkhook.WrapApp(raw), raw, nil
}

func TestPruneSnapshot_MissingDirReturnsError(t *testing.T) {
	err := PruneSnapshot("/no/such/dir", 1, testPruneFactory)
	require.Error(t, err)
	require.Contains(t, err.Error(), "open application.db")
}

func TestRegisterDefaultPruneFactory(t *testing.T) {
	original := DefaultPruneFactory()
	defer RegisterDefaultPruneFactory(original)

	RegisterDefaultPruneFactory(testPruneFactory)
	require.NotNil(t, DefaultPruneFactory())

	RegisterDefaultPruneFactory(nil)
	require.Nil(t, DefaultPruneFactory())
}
