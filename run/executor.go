package run

import (
	"fmt"
	"math/rand"
	"sort"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"

	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp/lifecycle"
	"github.com/cosmos/cosmos-sdk/baseapp/txnrunner"
	"github.com/cosmos/cosmos-sdk/client"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/runtime"
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
)

type txBuilderFn func(spec compare.TxSpec, keys map[string]cryptotypes.PrivKey) ([]sdk.Msg, error)

var (
	extraModuleOpts []configurator.ModuleOption
	extraTxBuilders = map[string]txBuilderFn{}
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
	oracle      *runtime.App
	probe       *runtime.App
	txConfig    client.TxConfig
	keys        map[string]cryptotypes.PrivKey
	accountNums map[string]uint64
	sequences   map[string]uint64
}

func NewFixtureExecutor() *FixtureExecutor {
	return &FixtureExecutor{}
}

func (e *FixtureExecutor) Init(genesis compare.GenesisSpec) error {
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
			return fmt.Errorf("parse balance for %s: %w", name, err)
		}
		genAccounts = append(genAccounts, simtestutil.GenesisAccount{
			GenesisAccount: acc,
			Coins:          coins,
		})
	}

	valSet, err := simtestutil.CreateRandomValidatorSet()
	if err != nil {
		return fmt.Errorf("create validator set: %w", err)
	}

	baseCfg := simtestutil.StartupConfig{
		ValidatorSet:    func() (*cmttypes.ValidatorSet, error) { return valSet, nil },
		AtGenesis:       true,
		GenesisAccounts: genAccounts,
	}

	var txCfg client.TxConfig

	baseCfg.DB = dbm.NewMemDB()
	cfg := buildAppConfig()
	oracleApp, err := simtestutil.SetupWithConfiguration(cfg, baseCfg, &txCfg)
	if err != nil {
		return fmt.Errorf("setup oracle app: %w", err)
	}
	instrument.InstrumentApp(oracleApp, instrument.Options{Runner: instrument.RunnerSequential})

	baseCfg.DB = dbm.NewMemDB()
	probeApp, err := simtestutil.SetupWithConfiguration(cfg, baseCfg)
	if err != nil {
		return fmt.Errorf("setup probe app: %w", err)
	}
	instrument.InstrumentSTM(probeApp, txnrunner.NewSTMRunner(txCfg.TxDecoder(), nil, 4, false, nil))

	e.oracle = oracleApp
	e.probe = probeApp
	e.txConfig = txCfg
	e.keys = keys
	e.accountNums = accountNums
	e.sequences = sequences

	return nil
}

func (e *FixtureExecutor) RunBlock(block compare.BlockSpec, height int64) (*compare.Result, error) {
	var txs [][]byte
	for _, spec := range block.Txs {
		txBytes, err := e.buildTx(spec)
		if err != nil {
			return nil, fmt.Errorf("build tx (signer=%s, msg=%s): %w", spec.Signer, spec.Msg, err)
		}
		txs = append(txs, txBytes)
	}

	oracleObs := compare.NewBlockObserver(len(txs))
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
		OracleWriteSets: oracleObs,
		ProbeWriteSets:  probeObs,
	})

	e.oracle.SetLifecycleObserver(lifecycle.NoopLifecycleObserver{})
	e.probe.SetLifecycleObserver(lifecycle.NoopLifecycleObserver{})

	return result, err
}

func (e *FixtureExecutor) Close() {
	e.oracle = nil
	e.probe = nil
	e.txConfig = nil
	e.keys = nil
	e.accountNums = nil
	e.sequences = nil
}

func (e *FixtureExecutor) buildTx(spec compare.TxSpec) ([]byte, error) {
	fromKey, ok := e.keys[spec.Signer]
	if !ok {
		return nil, fmt.Errorf("unknown signer %q", spec.Signer)
	}

	var msgs []sdk.Msg
	switch spec.Msg {
	case "bank-send":
		toKey, ok := e.keys[spec.To]
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
			msgs, err = builder(spec, e.keys)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("unsupported message type %q", spec.Msg)
		}
	}

	accNum := e.accountNums[spec.Signer]
	seq := e.sequences[spec.Signer]
	e.sequences[spec.Signer] = seq + 1

	tx, err := simtestutil.GenSignedMockTx(
		rand.New(rand.NewSource(42)),
		e.txConfig,
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

	return e.txConfig.TxEncoder()(tx)
}

func sortedAccountNames(m map[string]compare.AccountSpec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
