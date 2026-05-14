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

	// Owned resources released in Close(). oracleDB is opened from the snapshot
	// directory; probeDB is opened from a temp-dir copy so the two apps can
	// commit independently without trampling each other's IAVL versions.
	oracleDB    dbm.DB
	probeDB     dbm.DB
	probeDBTemp string
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

// InitFromState creates oracle and probe app instances backed by separate
// application.db handles, then pins both to IAVL version meta.Start - 1 so
// that the harness's first FinalizeBlock(meta.Start) is accepted.
//
// Probe isolation: goleveldb takes an exclusive LOCK on its directory, so we
// cannot open the same application.db twice. Instead we copy application.db
// into a temp directory and open the copy for the probe. After Commit, oracle
// and probe diverge into separate physical stores (which is required because
// they each commit IAVL version meta.Start with potentially different writes).
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

	oracleDB, err := openApplicationDB(snapshotDir)
	if err != nil {
		return fmt.Errorf("open oracle application.db: %w", err)
	}
	e.oracleDB = oracleDB

	probeDir, err := os.MkdirTemp("", "blockstm-sim-probe-*")
	if err != nil {
		e.closeOracleDB()
		return fmt.Errorf("create probe temp dir: %w", err)
	}
	e.probeDBTemp = probeDir

	srcAppDB := filepath.Join(snapshotDir, applicationDBName+".db")
	dstAppDB := filepath.Join(probeDir, applicationDBName+".db")
	if err := copyDir(srcAppDB, dstAppDB); err != nil {
		e.cleanup()
		return fmt.Errorf("copy application.db to probe temp dir: %w", err)
	}

	probeDB, err := openApplicationDB(probeDir)
	if err != nil {
		e.cleanup()
		return fmt.Errorf("open probe application.db: %w", err)
	}
	e.probeDB = probeDB

	var txCfg client.TxConfig

	oracleApp, rawOracleApp, err := e.appFactoryFn(compare.GenesisSpec{})(oracleDB, &txCfg)
	if err != nil {
		e.cleanup()
		return fmt.Errorf("setup oracle app: %w", err)
	}
	if err := oracleApp.LoadVersion(loadVersion); err != nil {
		e.cleanup()
		return fmt.Errorf("oracle LoadVersion(%d): %w", loadVersion, err)
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
	if err := probeApp.LoadVersion(loadVersion); err != nil {
		e.cleanup()
		return fmt.Errorf("probe LoadVersion(%d): %w", loadVersion, err)
	}
	instrument.InstrumentSTM(probeApp, sdkhook.NewSTMRunner(
		txCfg.TxDecoder(), probeApp.GetStoreKeys(), 4, rand.Int63(),
	))

	e.oracle = oracleApp
	e.probe = probeApp
	e.txConfig = txCfg
	return nil
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
		result.MsgKeys = make([]string, len(txs))
		for i := range result.MsgKeys {
			result.MsgKeys[i] = "raw"
		}
	}

	return result, err
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
