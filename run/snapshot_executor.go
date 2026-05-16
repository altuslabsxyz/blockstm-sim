//go:build sdk_hooks

package run

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/instrument"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
	"github.com/altuslabsxyz/blockstm-sim/simharness"
	"github.com/altuslabsxyz/blockstm-sim/tracker"
)

// applicationDBName is the on-disk directory name (under a snapshot dir) that
// holds the IAVL multistore. It matches cosmos-sdk's BaseApp convention and
// the cp instructions emitted by `blockstm-sim extract`.
const applicationDBName = "application"

// SnapshotExecutor executes mainnet blocks against pre-block state loaded from
// application.db. It implements both Executor and StateInitializer so that
// RunHarness can drive it with a SnapshotCorpus.
//
// Usage in chain-side test:
//
//	executor := run.NewSnapshotExecutor(run.WithSnapshotAppFactoryFunc(chainFactory))
//	run.RunHarness(cfg, executor, snapshotStores, rep, os.Stderr)
type SnapshotExecutor struct {
	oracle         sdkhook.App
	probe          sdkhook.App
	txConfig       client.TxConfig
	oracleTrackers []compare.MutationTracker
	appFactoryFn   AppFactoryFunc

	// Owned resources released in Close(). Both oracle and probe operate on
	// temp-dir copies of application.db so that (a) the two apps can commit
	// independently without trampling each other's IAVL versions and (b) the
	// original snapshot directory is never mutated, keeping it reusable across
	// test runs.
	oracleDB     dbm.DB
	probeDB      dbm.DB
	oracleDBTemp string
	probeDBTemp  string
}

func NewSnapshotExecutor(opts ...func(*SnapshotExecutor)) *SnapshotExecutor {
	e := &SnapshotExecutor{}
	for _, o := range opts {
		o(e)
	}
	return e
}

// WithSnapshotAppFactoryFunc sets the genesis-aware factory. For snapshot
// corpora the GenesisSpec passed to fn is always empty — the app is
// initialised from application.db via LoadVersion(meta.Start - 1), not from
// genesis.
func WithSnapshotAppFactoryFunc(fn AppFactoryFunc) func(*SnapshotExecutor) {
	return func(e *SnapshotExecutor) { e.appFactoryFn = fn }
}

// Init is not supported for snapshot corpora.
func (e *SnapshotExecutor) Init(_ compare.GenesisSpec) error {
	return fmt.Errorf("SnapshotExecutor does not support Init; use a SnapshotCorpus which triggers InitFromState")
}

// InitFromState creates oracle and probe app instances backed by independent
// temp-dir copies of application.db, then pins each to IAVL version
// meta.Start - 1 so that the harness's first FinalizeBlock(meta.Start) is
// accepted.
//
// Why two copies: goleveldb takes an exclusive LOCK on its directory, so we
// cannot open the same application.db twice. We also cannot reuse the
// snapshot's original application.db for either side — a snapshot produced by
// `blockstm-sim extract` from a live node contains every IAVL version up
// through meta.End, and re-committing at version meta.Start with a different
// hash (which is expected whenever the test binary differs from the
// production binary, e.g. `-tags test` globals on EVM chains) makes IAVL's
// SaveVersion panic with "version X was already saved to different hash".
//
// To avoid both, we first prune the source application.db in-place to
// meta.Start - 1 (a no-op if already pruned from a prior run), then copy
// the already-compact DB into two temp dirs — one per side. This order
// means each copy is smaller, so the two file-system copies are faster than
// the original copy-then-prune-each-copy approach.
func (e *SnapshotExecutor) InitFromState(snapshotDir string, meta compare.RangeMeta) error {
	if e.appFactoryFn == nil {
		return fmt.Errorf("SnapshotExecutor: AppFactoryFunc not set; call WithSnapshotAppFactoryFunc")
	}
	if snapshotDir == "" {
		return fmt.Errorf("SnapshotExecutor: snapshotDir is empty")
	}
	if meta.Start <= 0 {
		return fmt.Errorf("SnapshotExecutor: invalid meta.Start %d", meta.Start)
	}
	loadVersion := meta.Start - 1

	// Prune the source snapshot before copying so both copies start compact.
	// The pre-pruned guard in loadAndTruncate makes this a near-no-op on
	// subsequent runs.
	if loadVersion > 0 {
		factory := e.appFactoryFn(compare.GenesisSpec{})
		if err := PruneSnapshot(snapshotDir, loadVersion, factory); err != nil {
			return fmt.Errorf("prune source snapshot to version %d: %w", loadVersion, err)
		}
	}

	oracleDir, oracleDB, err := copyAndOpenApplicationDB(snapshotDir, "blockstm-sim-oracle-*")
	if err != nil {
		return fmt.Errorf("stage oracle application.db: %w", err)
	}
	e.oracleDBTemp = oracleDir
	e.oracleDB = oracleDB

	probeDir, probeDB, err := copyAndOpenApplicationDB(snapshotDir, "blockstm-sim-probe-*")
	if err != nil {
		e.cleanup()
		return fmt.Errorf("stage probe application.db: %w", err)
	}
	e.probeDBTemp = probeDir
	e.probeDB = probeDB

	var txCfg client.TxConfig

	oracleApp, rawOracleApp, err := e.appFactoryFn(compare.GenesisSpec{})(oracleDB, &txCfg)
	if err != nil {
		e.cleanup()
		return fmt.Errorf("setup oracle app: %w", err)
	}
	if err := loadAndTruncate(oracleApp, loadVersion); err != nil {
		e.cleanup()
		return fmt.Errorf("oracle init at version %d: %w", loadVersion, err)
	}
	instrument.InstrumentSTM(oracleApp, sdkhook.NewSTMRunner(
		txCfg.TxDecoder(), oracleApp.GetStoreKeys(), 1, 0,
	))
	for _, mod := range sdkhook.DiscoverKeepers(rawOracleApp) {
		t := tracker.New(mod)
		if tracker.ShouldSkipTracker(t.TrackerName()) {
			continue
		}
		e.oracleTrackers = append(e.oracleTrackers, t)
	}

	probeApp, _, err := e.appFactoryFn(compare.GenesisSpec{})(probeDB, nil)
	if err != nil {
		e.cleanup()
		return fmt.Errorf("setup probe app: %w", err)
	}
	if err := loadAndTruncate(probeApp, loadVersion); err != nil {
		e.cleanup()
		return fmt.Errorf("probe init at version %d: %w", loadVersion, err)
	}
	instrument.InstrumentSTM(probeApp, sdkhook.NewSTMRunner(
		txCfg.TxDecoder(), probeApp.GetStoreKeys(), 4, rand.Int63(),
	))

	e.oracle = oracleApp
	e.probe = probeApp
	e.txConfig = txCfg
	return nil
}

// loadAndTruncate pins the app's multistore at loadVersion and discards every
// IAVL version above it. The truncation step is what makes the subsequent
// Commit at loadVersion+1 safe: without it, IAVL.SaveVersion would refuse to
// overwrite the existing committed version (and panic with a hash-mismatch
// error if the test binary's IAVL hash diverges from the original node's).
//
// CommitMultiStore.RollbackToVersion is the cosmos-sdk's canonical API for
// this — it calls each IAVL store's MutableTree.LoadVersionForOverwriting,
// which in turn invokes ndb.DeleteVersionsFrom. That function handles both
// legacy (pre-v1, `'n'`/`'r'` prefixed) and v1 (`'s'` prefixed) IAVL key
// formats correctly, so this works on snapshots from any node that's gone
// through the IAVL v0→v1 migration as well as freshly-built v1 nodes.
//
// LoadVersion is called first so that rs.stores is populated and BaseApp's
// internal state is initialised; RollbackToVersion then prunes and re-loads
// the latest version (which is now loadVersion, since everything above it
// has been deleted).
func loadAndTruncate(app sdkhook.App, loadVersion int64) error {
	if err := app.LoadVersion(loadVersion); err != nil {
		return fmt.Errorf("LoadVersion: %w", err)
	}
	// rootmulti.Store.RollbackToVersion rejects target <= 0. When loadVersion
	// is 0 (meta.Start == 1) there is nothing above to prune anyway, so skip.
	// Also skip when the store's latest committed version already equals
	// loadVersion — the DB has been pre-pruned (e.g. by PruneSnapshot) and
	// RollbackToVersion would be a no-op scan.
	if loadVersion > 0 && app.CommitMultiStore().LatestVersion() > loadVersion {
		if err := app.CommitMultiStore().RollbackToVersion(loadVersion); err != nil {
			return fmt.Errorf("RollbackToVersion: %w", err)
		}
	}
	return nil
}

// copyAndOpenApplicationDB creates a fresh temp directory, copies the
// snapshot's application.db into it, and opens the copy. The caller owns
// both the returned directory (to remove on cleanup) and the DB handle (to
// close on cleanup). On error, any partial state is cleaned up internally.
func copyAndOpenApplicationDB(snapshotDir, tempPattern string) (string, dbm.DB, error) {
	tempDir, err := os.MkdirTemp("", tempPattern)
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	src := filepath.Join(snapshotDir, applicationDBName+".db")
	dst := filepath.Join(tempDir, applicationDBName+".db")
	if err := copyDir(src, dst); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("copy application.db: %w", err)
	}
	db, err := openApplicationDB(tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, fmt.Errorf("open application.db: %w", err)
	}
	return tempDir, db, nil
}

// RunBlock runs a single mainnet block through the F1 comparison pipeline.
// block.RawTxs carries the pre-signed bytes from SnapshotCorpus.
func (e *SnapshotExecutor) RunBlock(block compare.BlockSpec, height int64) (*compare.Result, error) {
	if e.oracle == nil {
		return nil, fmt.Errorf("SnapshotExecutor not initialised; InitFromState must be called first")
	}

	txs := block.RawTxs

	oracleTrackers := append([]compare.MutationTracker(nil), e.oracleTrackers...)
	oraclePreSnaps := make([][]byte, len(oracleTrackers))
	for i, t := range oracleTrackers {
		oraclePreSnaps[i] = t.SnapshotOutOfKVStoreState()
	}

	oracleObs := compare.NewBlockObserver(len(txs), oracleTrackers...)
	probeObs := compare.NewBlockObserver(len(txs))
	if setter, ok := simharness.Provider().(compare.TxIndexSetter); ok {
		oracleObs.AddTxSetter(setter)
	}
	e.oracle.SetLifecycleObserver(oracleObs)
	e.probe.SetLifecycleObserver(probeObs)

	result, err := compare.Run(compare.Input{
		Oracle:          e.oracle,
		Probe:           e.probe,
		Block:           &abci.RequestFinalizeBlock{Height: height, Txs: txs},
		OracleWriteSets: oracleObs,
		ProbeWriteSets:  probeObs,
		OracleMutations: oracleObs,
		NonDetProvider:  simharness.Provider(),
		PostOracleHook: func() {
			for i, t := range oracleTrackers {
				after := t.SnapshotOutOfKVStoreState()
				if !bytes.Equal(oraclePreSnaps[i], after) {
					oracleObs.AddBlockMutation(compare.MutationRecord{
						Tracker: t.TrackerName(),
						Before:  oraclePreSnaps[i],
						After:   after,
					})
				}
			}
		},
	})

	e.oracle.SetLifecycleObserver(compare.NoopLifecycleObserver{})
	e.probe.SetLifecycleObserver(compare.NoopLifecycleObserver{})

	if err == nil {
		if _, cerr := e.oracle.Commit(); cerr != nil {
			return nil, fmt.Errorf("oracle Commit: %w", cerr)
		}
		if _, cerr := e.probe.Commit(); cerr != nil {
			return nil, fmt.Errorf("probe Commit: %w", cerr)
		}
		result.MsgKeys = decodeMsgKeys(e.txConfig, txs)
	}

	return result, err
}

// decodeMsgKeys derives a per-tx coverage key from each raw tx. The key is
// the proto type URL of the first message (e.g. /cosmos.bank.v1beta1.MsgSend).
// Decode failures and empty-message txs fall back to "raw" — EVM-native or
// otherwise unparseable txs land there, preserving the prior unconditional
// behaviour for that subset.
func decodeMsgKeys(txCfg client.TxConfig, txs [][]byte) []string {
	keys := make([]string, len(txs))
	decoder := txCfg.TxDecoder()
	for i, rawTx := range txs {
		keys[i] = "raw"
		tx, err := decoder(rawTx)
		if err != nil {
			continue
		}
		msgs := tx.GetMsgs()
		if len(msgs) == 0 {
			continue
		}
		keys[i] = sdk.MsgTypeURL(msgs[0])
	}
	return keys
}

func (e *SnapshotExecutor) Close() {
	e.oracle = nil
	e.probe = nil
	e.oracleTrackers = nil
	e.cleanup()
}

// openApplicationDB opens the goleveldb at <dir>/application.db.
// cosmos-db's NewGoLevelDB appends ".db" to the name argument, matching
// BaseApp's on-disk layout.
func openApplicationDB(dir string) (dbm.DB, error) {
	return dbm.NewGoLevelDB(applicationDBName, dir, nil)
}

func (e *SnapshotExecutor) closeOracleDB() {
	if e.oracleDB != nil {
		_ = e.oracleDB.Close()
		e.oracleDB = nil
	}
}

func (e *SnapshotExecutor) closeProbeDB() {
	if e.probeDB != nil {
		_ = e.probeDB.Close()
		e.probeDB = nil
	}
}

func (e *SnapshotExecutor) cleanup() {
	e.closeOracleDB()
	e.closeProbeDB()
	if e.oracleDBTemp != "" {
		_ = os.RemoveAll(e.oracleDBTemp)
		e.oracleDBTemp = ""
	}
	if e.probeDBTemp != "" {
		_ = os.RemoveAll(e.probeDBTemp)
		e.probeDBTemp = ""
	}
}

// copyDir recursively copies the contents of src into dst, creating dst if
// needed. Used to clone application.db so oracle and probe can each open an
// independent goleveldb without LOCK conflicts.
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: not a directory", src)
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
