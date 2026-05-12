//go:build sdk_hooks

package run

import (
	"bytes"
	"fmt"
	"math/rand"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/instrument"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
	"github.com/altuslabsxyz/blockstm-sim/simharness"
	"github.com/altuslabsxyz/blockstm-sim/tracker"
)

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
// initialised from preStateDB via CacheMultiStoreWithVersion, not from genesis.
func WithSnapshotAppFactoryFunc(fn AppFactoryFunc) func(*SnapshotExecutor) {
	return func(e *SnapshotExecutor) { e.appFactoryFn = fn }
}

// Init is not supported for snapshot corpora.
func (e *SnapshotExecutor) Init(_ compare.GenesisSpec) error {
	return fmt.Errorf("SnapshotExecutor does not support Init; use a SnapshotCorpus which triggers InitFromState")
}

// InitFromState creates oracle and probe app instances backed by preStateDB
// (application.db from the snapshot). Both apps load their IAVL state via
// CacheMultiStoreWithVersion so they start from the exact mainnet state at
// the snapshot's start height rather than from genesis.
func (e *SnapshotExecutor) InitFromState(preStateDB dbm.DB) error {
	if e.appFactoryFn == nil {
		return fmt.Errorf("SnapshotExecutor: AppFactoryFunc not set; call WithSnapshotAppFactoryFunc")
	}

	var txCfg client.TxConfig

	// Oracle — backed by preStateDB (the real application.db).
	oracleApp, rawOracleApp, err := e.appFactoryFn(compare.GenesisSpec{})(preStateDB, &txCfg)
	if err != nil {
		return fmt.Errorf("setup oracle app: %w", err)
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

	// Probe — backed by a separate MemDB so it has independent state.
	probeApp, _, err := e.appFactoryFn(compare.GenesisSpec{})(dbm.NewMemDB(), nil)
	if err != nil {
		return fmt.Errorf("setup probe app: %w", err)
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
}
