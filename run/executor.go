//go:build sdk_hooks

package run

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/configurator"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/cosmos/cosmos-sdk/x/auth"
	_ "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	_ "github.com/cosmos/cosmos-sdk/x/bank"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	_ "github.com/cosmos/cosmos-sdk/x/consensus"
	_ "github.com/cosmos/cosmos-sdk/x/distribution"
	_ "github.com/cosmos/cosmos-sdk/x/mint"
	_ "github.com/cosmos/cosmos-sdk/x/protocolpool"
	_ "github.com/cosmos/cosmos-sdk/x/staking"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/instrument"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
	"github.com/altuslabsxyz/blockstm-sim/simharness"
	"github.com/altuslabsxyz/blockstm-sim/tracker"
)

// initApp creates a fresh app with a new MemDB using the provided depinject
// config, startup config, and an optional pointer to receive the TxConfig.
func initApp(cfg depinject.Config, baseCfg simtestutil.StartupConfig, txCfgOut *client.TxConfig) (sdkhook.App, error) {
	baseCfg.DB = dbm.NewMemDB()
	if txCfgOut != nil {
		app, err := simtestutil.SetupWithConfiguration(cfg, baseCfg, txCfgOut)
		if err != nil {
			return nil, err
		}
		return sdkhook.WrapApp(app), nil
	}
	app, err := simtestutil.SetupWithConfiguration(cfg, baseCfg)
	if err != nil {
		return nil, err
	}
	return sdkhook.WrapApp(app), nil
}

// buildTx builds and signs a single transaction from a TxSpec.
// sequences[spec.Signer] is incremented as a side-effect.
func buildTx(
	spec compare.TxSpec,
	txConfig client.TxConfig,
	keys map[string]cryptotypes.PrivKey,
	accountNums map[string]uint64,
	sequences map[string]uint64,
	builders map[string]TxBuilderFn,
) ([]byte, error) {
	fromKey, ok := keys[spec.Signer]
	if !ok {
		return nil, fmt.Errorf("unknown signer %q", spec.Signer)
	}

	var msgs []sdk.Msg
	switch spec.Msg {
	case "bank-send":
		toKey, ok := keys[spec.To]
		if !ok {
			return nil, fmt.Errorf("unknown recipient %q", spec.To)
		}
		amount, err := sdk.ParseCoinsNormalized(spec.Amount)
		if err != nil {
			return nil, fmt.Errorf("parse amount: %w", err)
		}
		msgs = append(msgs, banktypes.NewMsgSend(
			fromKey.PubKey().Address().Bytes(),
			toKey.PubKey().Address().Bytes(),
			amount,
		))
	default:
		if builder, ok := builders[spec.Msg]; ok {
			var err error
			msgs, err = builder(spec, keys)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("unsupported message type %q", spec.Msg)
		}
	}

	accNum := accountNums[spec.Signer]
	seq := sequences[spec.Signer]
	sequences[spec.Signer] = seq + 1

	tx, err := simtestutil.GenSignedMockTx(
		rand.New(rand.NewSource(42)),
		txConfig,
		msgs,
		sdk.NewCoins(sdk.NewCoin("stake", sdkmath.NewInt(0))),
		spec.Gas,
		"",
		[]uint64{accNum},
		[]uint64{seq},
		fromKey,
	)
	if err != nil {
		return nil, fmt.Errorf("sign tx: %w", err)
	}

	return txConfig.TxEncoder()(tx)
}

// TxBuilderFn builds SDK messages from a TxSpec for a given msg type key.
// Register via WithExtraTxBuilders on FixtureExecutor.
type TxBuilderFn func(spec compare.TxSpec, keys map[string]cryptotypes.PrivKey) ([]sdk.Msg, error)

var (
	extraModuleOpts            []configurator.ModuleOption
	extraTxBuilders            = map[string]TxBuilderFn{}
	extraOracleOutputs         []any
	extraOracleBlockCtxTracker func(height int64) *compare.BlockContextTracker
	extraPreOracleSetup        func()
	// extraPreProbeSetup is called inside PostOracleHook (between oracle and
	// probe FinalizeBlock) to configure any probe-specific state before the
	// probe executes.
	extraPreProbeSetup func()
	// extraPostRunBlockHook is called after compare.Run returns in RunBlock to
	// clean up any block-scoped state set by extraPreProbeSetup.
	extraPostRunBlockHook func()
)

func buildAppConfig() depinject.Config {
	opts := []configurator.ModuleOption{
		configurator.AuthModule(),
		configurator.BankModule(),
		configurator.StakingModule(),
		configurator.MintModule(),
		configurator.DistributionModule(),
		configurator.ProtocolPoolModule(),
		configurator.ConsensusModule(),
		configurator.TxModule(),
	}
	opts = append(opts, extraModuleOpts...)
	return depinject.Configs(
		configurator.NewAppConfig(opts...),
		depinject.Supply(log.NewNopLogger()),
	)
}

// AppFactory creates and initialises a fresh app backed by db.
// If txCfgOut is non-nil, the factory must populate it with the app's TxConfig.
// outputs captures additional dependency-injection targets (e.g. keeper pointers);
// chain-side factories that do not use depinject may ignore them.
// The second return value is the raw (unwrapped) app passed to sdkhook.DiscoverKeepers.
type AppFactory func(db dbm.DB, txCfgOut *client.TxConfig, outputs ...any) (sdkhook.App, any, error)

// AppFactoryFunc produces an AppFactory bound to a specific genesis. Called
// once per Init so each fixture gets an app bootstrapped from its own accounts.
// Use WithAppFactoryFunc when fixtures in the corpus have different account sets
// (e.g. bank-send fixtures use sender/receiver; canary fixtures use alice/bob).
type AppFactoryFunc func(genesis compare.GenesisSpec) AppFactory

type FixtureExecutor struct {
	oracle        sdkhook.App
	probe         sdkhook.App
	txConfig      client.TxConfig
	keys          map[string]cryptotypes.PrivKey
	accountNums   map[string]uint64
	sequences     map[string]uint64
	oracleWorkers int // 0 = 1-worker BlockSTM (deterministic); >0 = BlockSTM with N workers
	// oracleTrackers holds the out-of-KVStore mutation trackers for the oracle app.
	// Populated via reflect-based discovery after oracle setup in Init.
	oracleTrackers []compare.MutationTracker
	// appFactory, when non-nil, overrides the default simtestutil-based app construction.
	// Set via WithAppFactory to wire a real chain app (e.g. stable.NewApp()).
	appFactory AppFactory
	// appFactoryFn, when non-nil, is called in Init with the fixture's genesis to
	// produce an AppFactory. Takes precedence over appFactory.
	appFactoryFn AppFactoryFunc
	// extraTxBuilders holds instance-level builders registered via WithExtraTxBuilders.
	extraTxBuilders map[string]TxBuilderFn
}

// WithSTMOracle configures the oracle to use BlockSTM with more than one worker.
func WithSTMOracle(workers int) func(*FixtureExecutor) {
	return func(e *FixtureExecutor) { e.oracleWorkers = workers }
}

// WithAppFactory sets a custom app construction function on the executor.
// Use this to wire a real chain app (e.g. stable.NewApp()) instead of the
// default simtestutil-based test app. See docs/integration.md for an example.
func WithAppFactory(f AppFactory) func(*FixtureExecutor) {
	return func(e *FixtureExecutor) { e.appFactory = f }
}

// WithAppFactoryFunc sets a genesis-aware factory function on the executor.
// fn is called in Init with the fixture's GenesisSpec, returning an AppFactory
// bound to that genesis. This avoids the need for a per-fixture executor wrapper
// when the corpus contains fixtures with different account sets.
func WithAppFactoryFunc(fn AppFactoryFunc) func(*FixtureExecutor) {
	return func(e *FixtureExecutor) { e.appFactoryFn = fn }
}

// WithExtraTxBuilders registers custom message builders on the executor.
// Keys are the msg type strings used in TxSpec.Msg (e.g. "gov-vote").
// Instance-level builders take precedence over package-level ones registered
// via init() (e.g. canary builders).
func WithExtraTxBuilders(builders map[string]TxBuilderFn) func(*FixtureExecutor) {
	return func(e *FixtureExecutor) { e.extraTxBuilders = builders }
}

func NewFixtureExecutor(opts ...func(*FixtureExecutor)) *FixtureExecutor {
	fe := &FixtureExecutor{}
	for _, o := range opts {
		o(fe)
	}
	return fe
}

// mergedTxBuilders returns a map that combines package-level extraTxBuilders
// (registered via init, e.g. canary) and instance-level e.extraTxBuilders.
// Instance-level entries take precedence over package-level ones.
func (e *FixtureExecutor) mergedTxBuilders() map[string]TxBuilderFn {
	merged := make(map[string]TxBuilderFn, len(extraTxBuilders)+len(e.extraTxBuilders))
	for k, v := range extraTxBuilders {
		merged[k] = v
	}
	for k, v := range e.extraTxBuilders {
		merged[k] = v
	}
	return merged
}

func (e *FixtureExecutor) Init(genesis compare.GenesisSpec) error {
	// appFactoryFn takes precedence: it binds a fresh AppFactory to this
	// fixture's genesis so each Init gets an app with the correct accounts.
	if e.appFactoryFn != nil {
		e.appFactory = e.appFactoryFn(genesis)
	}

	gs, err := initGenesis(genesis)
	if err != nil {
		return err
	}

	var txCfg client.TxConfig
	cfg := buildAppConfig()

	if extraPreOracleSetup != nil {
		extraPreOracleSetup()
	}

	var oracleApp sdkhook.App
	var rawOracleApp any

	if e.appFactory != nil {
		oracleApp, rawOracleApp, err = e.appFactory(dbm.NewMemDB(), &txCfg, extraOracleOutputs...)
		if err != nil {
			return fmt.Errorf("setup oracle app: %w", err)
		}
	} else {
		oracleOutputs := append([]any{&txCfg}, extraOracleOutputs...)
		gs.baseCfg.DB = dbm.NewMemDB()
		raw, err2 := simtestutil.SetupWithConfiguration(cfg, gs.baseCfg, oracleOutputs...)
		if err2 != nil {
			return fmt.Errorf("setup oracle app: %w", err2)
		}
		oracleApp = sdkhook.WrapApp(raw)
		rawOracleApp = raw
	}

	{
		workers := e.oracleWorkers
		if workers == 0 {
			workers = 1
		}
		instrument.InstrumentSTM(oracleApp, sdkhook.NewSTMRunner(txCfg.TxDecoder(), oracleApp.GetStoreKeys(), workers, 0))
	}

	// Populate generic reflect-based trackers for all discovered modules/keepers.
	// Skip trackers whose package prefix is in the deny list (e.g. upgrade module
	// whose downgradeVerified field transitions deterministically on every run).
	for _, mod := range sdkhook.DiscoverKeepers(rawOracleApp) {
		t := tracker.New(mod)
		if tracker.ShouldSkipTracker(t.TrackerName()) {
			continue
		}
		e.oracleTrackers = append(e.oracleTrackers, t)
	}

	var probeApp sdkhook.App
	if e.appFactory != nil {
		probeApp, _, err = e.appFactory(dbm.NewMemDB(), nil)
		if err != nil {
			return fmt.Errorf("setup probe app: %w", err)
		}
	} else {
		probeApp, err = initApp(cfg, gs.baseCfg, nil)
		if err != nil {
			return fmt.Errorf("setup probe app: %w", err)
		}
	}
	instrument.InstrumentSTM(probeApp, sdkhook.NewSTMRunner(txCfg.TxDecoder(), probeApp.GetStoreKeys(), 4, rand.Int63()))

	e.oracle = oracleApp
	e.probe = probeApp
	e.txConfig = txCfg
	e.keys = gs.keys
	e.accountNums = gs.accountNums
	e.sequences = gs.sequences

	return nil
}

func (e *FixtureExecutor) RunBlock(block compare.BlockSpec, height int64) (*compare.Result, error) {
	var txs [][]byte
	for _, spec := range block.Txs {
		txBytes, err := buildTx(spec, e.txConfig, e.keys, e.accountNums, e.sequences, e.mergedTxBuilders())
		if err != nil {
			return nil, fmt.Errorf("build tx (signer=%s, msg=%s): %w", spec.Signer, spec.Msg, err)
		}
		txs = append(txs, txBytes)
	}
	// RawTxs carries pre-signed bytes from SnapshotCorpus. When no TxSpecs
	// were decoded (snapshot blocks have no fixture YAML), use RawTxs directly.
	if len(txs) == 0 && len(block.RawTxs) > 0 {
		txs = append([][]byte(nil), block.RawTxs...)
	}

	var blockCtxTracker *compare.BlockContextTracker
	var blockCtxMutations compare.BlockContextMutationProvider
	if extraOracleBlockCtxTracker != nil {
		blockCtxTracker = extraOracleBlockCtxTracker(height)
		blockCtxMutations = blockCtxTracker
	}

	oracleTrackers := append([]compare.MutationTracker(nil), e.oracleTrackers...)
	if blockCtxTracker != nil {
		oracleTrackers = append(oracleTrackers, blockCtxTracker)
	}

	oraclePreSnaps := make([][]byte, len(oracleTrackers))
	for i, t := range oracleTrackers {
		oraclePreSnaps[i] = t.SnapshotOutOfKVStoreState()
	}

	oracleObs := compare.NewBlockObserver(len(txs), oracleTrackers...)
	probeObs := compare.NewBlockObserver(len(txs))
	// Wire the nondet sink as a TxIndexSetter so nondet findings carry the
	// correct tx attribution instead of always reporting TxIndex=-1.
	if setter, ok := simharness.Provider().(compare.TxIndexSetter); ok {
		oracleObs.AddTxSetter(setter)
	}
	e.oracle.SetLifecycleObserver(oracleObs)
	e.probe.SetLifecycleObserver(probeObs)

	result, err := compare.Run(compare.Input{
		Oracle: e.oracle,
		Probe:  e.probe,
		Block: &abci.RequestFinalizeBlock{
			Height: height,
			Txs:    txs,
		},
		OracleWriteSets:       oracleObs,
		ProbeWriteSets:        probeObs,
		OracleMutations:       oracleObs,
		BlockContextMutations: blockCtxMutations,
		NonDetProvider:        simharness.Provider(),
		PostOracleHook: func() {
			if extraPreProbeSetup != nil {
				extraPreProbeSetup()
			}
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

	if extraPostRunBlockHook != nil {
		extraPostRunBlockHook()
	}

	if err == nil {
		if _, cerr := e.oracle.Commit(); cerr != nil {
			return nil, fmt.Errorf("oracle Commit: %w", cerr)
		}
		if _, cerr := e.probe.Commit(); cerr != nil {
			return nil, fmt.Errorf("probe Commit: %w", cerr)
		}
	}

	if err == nil {
		if len(block.Txs) > 0 {
			result.MsgKeys = make([]string, len(block.Txs))
			result.TxWriteSets = make([][]string, len(block.Txs))
			for i, spec := range block.Txs {
				result.MsgKeys[i] = spec.Msg
				result.TxWriteSets[i] = oracleObs.TxWriteSet(i)
			}
		} else {
			// RawTxs path: no fixture TxSpecs, use "raw" as a placeholder so
			// MsgKeys length matches the number of executed transactions.
			result.MsgKeys = make([]string, len(txs))
			result.TxWriteSets = make([][]string, len(txs))
			for i := range result.MsgKeys {
				result.MsgKeys[i] = "raw"
				result.TxWriteSets[i] = oracleObs.TxWriteSet(i)
			}
		}
	}

	return result, err
}

func (e *FixtureExecutor) Close() {
	e.oracle = nil
	e.probe = nil
	e.txConfig = nil
	e.keys = nil
	e.accountNums = nil
	e.sequences = nil
	e.oracleTrackers = nil
}

func (e *FixtureExecutor) buildTx(spec compare.TxSpec) ([]byte, error) {
	return buildTx(spec, e.txConfig, e.keys, e.accountNums, e.sequences, e.mergedTxBuilders())
}

func sortedAccountNames(m map[string]compare.AccountSpec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// genesisSetup holds the account keying data and startup config produced by
// initGenesis, shared between FixtureExecutor and RepeatRunExecutor.
type genesisSetup struct {
	keys        map[string]cryptotypes.PrivKey
	accountNums map[string]uint64
	sequences   map[string]uint64
	baseCfg     simtestutil.StartupConfig
}

// initGenesis builds per-account keys, account numbers, and a StartupConfig
// from a GenesisSpec.
func initGenesis(genesis compare.GenesisSpec) (genesisSetup, error) {
	names := sortedAccountNames(genesis.Accounts)

	keys := make(map[string]cryptotypes.PrivKey, len(genesis.Accounts))
	accountNums := make(map[string]uint64, len(genesis.Accounts))
	sequences := make(map[string]uint64, len(genesis.Accounts))
	genAccounts := make([]simtestutil.GenesisAccount, 0, len(genesis.Accounts))

	for i, name := range names {
		priv := DeriveKey(name)
		keys[name] = priv
		accountNums[name] = uint64(i)

		acc := authtypes.NewBaseAccount(
			priv.PubKey().Address().Bytes(),
			priv.PubKey(),
			uint64(i),
			0,
		)
		coins, err := sdk.ParseCoinsNormalized(genesis.Accounts[name].Balance)
		if err != nil {
			return genesisSetup{}, fmt.Errorf("parse balance for %s: %w", name, err)
		}
		genAccounts = append(genAccounts, simtestutil.GenesisAccount{
			GenesisAccount: acc,
			Coins:          coins,
		})
	}

	valSet, err := simtestutil.CreateRandomValidatorSet()
	if err != nil {
		return genesisSetup{}, fmt.Errorf("create validator set: %w", err)
	}

	baseCfg := simtestutil.StartupConfig{
		ValidatorSet:    func() (*cmttypes.ValidatorSet, error) { return valSet, nil },
		AtGenesis:       true,
		GenesisAccounts: genAccounts,
	}

	return genesisSetup{
		keys:        keys,
		accountNums: accountNums,
		sequences:   sequences,
		baseCfg:     baseCfg,
	}, nil
}
