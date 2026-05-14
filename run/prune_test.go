//go:build sdk_hooks

package run

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// testPruneFactory returns an AppFactory suitable for PruneSnapshot calls in
// tests. It opens the app on the provided DB using LoadLatestVersion (AtGenesis:
// false); loadAndTruncate then reloads at the precise target version.
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

// bootstrapSnapshotDB creates snapshotDir/application.db with 3 committed
// IAVL versions: genesis (version 1) plus two bare Commit() calls.
// Returns the LatestVersion observed before close.
func bootstrapSnapshotDB(t *testing.T, snapshotDir string) int64 {
	t.Helper()
	db, err := openApplicationDB(snapshotDir)
	require.NoError(t, err)

	valSet, err := simtestutil.CreateRandomValidatorSet()
	require.NoError(t, err)

	// GenesisStateWithValSet panics when GenAccounts is empty; supply one account.
	priv := DeriveKey("prune-test")
	acc := authtypes.NewBaseAccount(priv.PubKey().Address().Bytes(), priv.PubKey(), 0, 0)
	genAccounts := []simtestutil.GenesisAccount{
		{GenesisAccount: acc, Coins: sdk.NewCoins(sdk.NewCoin("stake", sdkmath.NewInt(10_000_000)))},
	}

	baseCfg := simtestutil.StartupConfig{
		DB:              db,
		AtGenesis:       true,
		ValidatorSet:    func() (*cmttypes.ValidatorSet, error) { return valSet, nil },
		GenesisAccounts: genAccounts,
	}
	raw, err := simtestutil.SetupWithConfiguration(buildAppConfig(), baseCfg)
	require.NoError(t, err)
	app := sdkhook.WrapApp(raw)

	// AtGenesis setup leaves app at version 0; commit three times to reach version 3.
	app.CommitMultiStore().Commit()
	app.CommitMultiStore().Commit()
	app.CommitMultiStore().Commit()

	latest := app.CommitMultiStore().LatestVersion()
	require.NoError(t, db.Close())
	return latest
}

func TestPruneSnapshot_PrunesVersionsAboveTarget(t *testing.T) {
	snapshotDir := t.TempDir()

	latest := bootstrapSnapshotDB(t, snapshotDir)
	require.Equal(t, int64(3), latest)

	// Prune to version 1.
	require.NoError(t, PruneSnapshot(snapshotDir, 1, testPruneFactory))

	// Re-open and verify LatestVersion is now 1.
	db, err := openApplicationDB(snapshotDir)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	app, _, err := testPruneFactory(db, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), app.CommitMultiStore().LatestVersion())
}

func TestPruneSnapshot_PrePrunedDBIsNoOp(t *testing.T) {
	snapshotDir := t.TempDir()
	bootstrapSnapshotDB(t, snapshotDir)

	// First prune: removes versions 2 and 3.
	require.NoError(t, PruneSnapshot(snapshotDir, 1, testPruneFactory))

	// Second prune at the same target: must succeed (already pre-pruned path).
	require.NoError(t, PruneSnapshot(snapshotDir, 1, testPruneFactory))

	db, err := openApplicationDB(snapshotDir)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	app, _, err := testPruneFactory(db, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), app.CommitMultiStore().LatestVersion())
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
