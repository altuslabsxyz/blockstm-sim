//go:build sdk_hooks

package run

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// stubCMS is a minimal CommitMultiStore for testing loadAndTruncate in isolation.
// It simulates the rootmulti behaviour where LatestVersion() reads from the DB
// (dbLatest) before LoadVersion is called, and from lastCommitInfo (loaded) after.
type stubCMS struct {
	storetypes.CommitMultiStore
	dbLatest   int64 // returned before loadCalled; simulates GetLatestVersion(db)
	loaded     int64 // returned after loadCalled; simulates lastCommitInfo.Version
	loadCalled bool
	rolledBack int64 // last RollbackToVersion target, 0 if never called
}

func (s *stubCMS) LatestVersion() int64 {
	if !s.loadCalled {
		return s.dbLatest
	}
	return s.loaded
}

func (s *stubCMS) RollbackToVersion(version int64) error {
	s.rolledBack = version
	s.loaded = version
	return nil
}

// stubApp implements sdkhook.App using stubCMS.
// Only CommitMultiStore and LoadVersion are used by loadAndTruncate.
type stubApp struct{ cms *stubCMS }

func (a *stubApp) CommitMultiStore() storetypes.CommitMultiStore { return a.cms }
func (a *stubApp) LoadVersion(v int64) error {
	a.cms.loadCalled = true
	a.cms.loaded = v
	return nil
}
func (a *stubApp) FinalizeBlock(*abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	panic("not used")
}
func (a *stubApp) Commit() (*abci.ResponseCommit, error)           { panic("not used") }
func (a *stubApp) SetLifecycleObserver(compare.LifecycleObserver)  {}
func (a *stubApp) SetBlockSTMTxRunner(sdkhook.STMRunner)           {}
func (a *stubApp) UnsetBlockSTMTxRunner()                          {}
func (a *stubApp) SetDisableBlockGasMeter(bool)                    {}
func (a *stubApp) GetStoreKeys() []storetypes.StoreKey             { return nil }

// TestLoadAndTruncate_RollsBackWhenDBAboveTarget is the regression test for
// the bug where LoadVersion(n) sets LatestVersion()=n, causing the
// `LatestVersion() > n` guard to always be false and skipping RollbackToVersion.
func TestLoadAndTruncate_RollsBackWhenDBAboveTarget(t *testing.T) {
	// DB has version 100 (production node committed up to 100).
	// Target is version 50 (we want to truncate to 50).
	// Before fix: LoadVersion(50) → LatestVersion()=50 → 50>50=false → no rollback.
	// After fix:  dbLatest=100 captured before LoadVersion → 100>50 → rollback.
	cms := &stubCMS{dbLatest: 100}
	app := &stubApp{cms: cms}

	require.NoError(t, loadAndTruncate(app, 50))

	require.Equal(t, int64(50), cms.rolledBack, "RollbackToVersion must be called with target version")
	require.Equal(t, int64(50), cms.LatestVersion())
}

func TestLoadAndTruncate_SkipsRollbackWhenAlreadyAtTarget(t *testing.T) {
	// DB is pre-pruned: dbLatest already equals loadVersion.
	cms := &stubCMS{dbLatest: 50}
	app := &stubApp{cms: cms}

	require.NoError(t, loadAndTruncate(app, 50))

	require.Equal(t, int64(0), cms.rolledBack, "RollbackToVersion must NOT be called when DB is already at target")
}

func TestLoadAndTruncate_SkipsRollbackForZeroVersion(t *testing.T) {
	// loadVersion=0 (meta.Start==1 case); rootmulti rejects RollbackToVersion(0).
	cms := &stubCMS{dbLatest: 0}
	app := &stubApp{cms: cms}

	require.NoError(t, loadAndTruncate(app, 0))

	require.Equal(t, int64(0), cms.rolledBack)
}

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
