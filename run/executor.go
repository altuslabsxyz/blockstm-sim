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
		if builder, ok := extraTxBuilders[spec.Msg]; ok {
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

type txBuilderFn func(spec compare.TxSpec, keys map[string]cryptotypes.PrivKey) ([]sdk.Msg, error)

var (
	extraModuleOpts            []configurator.ModuleOption
	extraTxBuilders            = map[string]txBuilderFn{}
	extraOracleOutputs         []any
	extraOracleMutTrackers     func() []compare.MutationTracker
	extraOracleBlockCtxTracker func(height int64) *compare.BlockContextTracker
	extraPreOracleSetup        func()
	// extraPopulateOracleTrackers is called after oracle app setup in Init to
	// populate the executor's oracleTrackers field from any registered keepers.
	extraPopulateOracleTrackers func(*FixtureExecutor)
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

type FixtureExecutor struct {
	oracle        sdkhook.App
	probe         sdkhook.App
	txConfig      client.TxConfig
	keys          map[string]cryptotypes.PrivKey
	accountNums   map[string]uint64
	sequences     map[string]uint64
	oracleWorkers int // 0 = 1-worker BlockSTM (deterministic); >0 = BlockSTM with N workers
	// oracleTrackers holds the out-of-KVStore mutation trackers for the oracle app.
	// Populated by extraPopulateOracleTrackers after oracle setup in Init.
	oracleTrackers []compare.MutationTracker
}

// WithSTMOracle configures the oracle to use BlockSTM with more than one worker.
func WithSTMOracle(workers int) func(*FixtureExecutor) {
	return func(e *FixtureExecutor) { e.oracleWorkers = workers }
}

func NewFixtureExecutor(opts ...func(*FixtureExecutor)) *FixtureExecutor {
	fe := &FixtureExecutor{}
	for _, o := range opts {
		o(fe)
	}
	return fe
}

func (e *FixtureExecutor) Init(genesis compare.GenesisSpec) error {
	gs, err := initGenesis(genesis)
	if err != nil {
		return err
	}

	var txCfg client.TxConfig

	cfg := buildAppConfig()
	gs.baseCfg.DB = dbm.NewMemDB()
	if extraPreOracleSetup != nil {
		extraPreOracleSetup()
	}
	oracleOutputs := append([]any{&txCfg}, extraOracleOutputs...)
	rawOracleApp, err := simtestutil.SetupWithConfiguration(cfg, gs.baseCfg, oracleOutputs...)
	if err != nil {
		return fmt.Errorf("setup oracle app: %w", err)
	}
	oracleApp := sdkhook.WrapApp(rawOracleApp)
	{
		workers := e.oracleWorkers
		if workers == 0 {
			workers = 1
		}
		instrument.InstrumentSTM(oracleApp, sdkhook.NewSTMRunner(txCfg.TxDecoder(), oracleApp.GetStoreKeys(), workers, 0))
	}

	if extraPopulateOracleTrackers != nil {
		extraPopulateOracleTrackers(e)
	}

	probeApp, err := initApp(cfg, gs.baseCfg, nil)
	if err != nil {
		return fmt.Errorf("setup probe app: %w", err)
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
		txBytes, err := buildTx(spec, e.txConfig, e.keys, e.accountNums, e.sequences)
		if err != nil {
			return nil, fmt.Errorf("build tx (signer=%s, msg=%s): %w", spec.Signer, spec.Msg, err)
		}
		txs = append(txs, txBytes)
	}

	var blockCtxTracker *compare.BlockContextTracker
	if extraOracleBlockCtxTracker != nil {
		blockCtxTracker = extraOracleBlockCtxTracker(height)
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
		BlockContextMutations: blockCtxTracker,
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
		result.MsgKeys = make([]string, len(block.Txs))
		for i, spec := range block.Txs {
			result.MsgKeys[i] = spec.Msg
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
	return buildTx(spec, e.txConfig, e.keys, e.accountNums, e.sequences)
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
