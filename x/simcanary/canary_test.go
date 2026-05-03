//go:build simharness_canary

package simcanary_test

import (
	"math/rand"
	"sort"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	appv1alpha1 "cosmossdk.io/api/cosmos/app/v1alpha1"
	"cosmossdk.io/core/appconfig"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil/configurator"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/cosmos/cosmos-sdk/x/auth"
	_ "github.com/cosmos/cosmos-sdk/x/auth/tx/config"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	_ "github.com/cosmos/cosmos-sdk/x/bank"
	_ "github.com/cosmos/cosmos-sdk/x/consensus"
	_ "github.com/cosmos/cosmos-sdk/x/distribution"
	_ "github.com/cosmos/cosmos-sdk/x/mint"
	_ "github.com/cosmos/cosmos-sdk/x/protocolpool"
	_ "github.com/cosmos/cosmos-sdk/x/staking"

	_ "github.com/altuslabsxyz/blockstm-sim/x/simcanary"
	simcanarytypes "github.com/altuslabsxyz/blockstm-sim/x/simcanary/types"
)

func canaryModule() configurator.ModuleOption {
	return func(config *configurator.Config) {
		config.ModuleConfigs[simcanarytypes.ModuleName] = &appv1alpha1.ModuleConfig{
			Name:   simcanarytypes.ModuleName,
			Config: appconfig.WrapAny(&simcanarytypes.Module{}),
		}
	}
}

func testAppConfig() depinject.Config {
	return depinject.Configs(
		configurator.NewAppConfig(
			configurator.AuthModule(),
			configurator.BankModule(),
			configurator.StakingModule(),
			configurator.MintModule(),
			configurator.DistributionModule(),
			configurator.ProtocolPoolModule(),
			configurator.ConsensusModule(),
			configurator.TxModule(),
			canaryModule(),
		),
		depinject.Supply(log.NewNopLogger()),
	)
}

type testKeys struct {
	keys  map[string]cryptotypes.PrivKey
	accNs map[string]uint64
}

func setupApp(t *testing.T) (*runtime.App, client.TxConfig, testKeys) {
	t.Helper()

	accounts := map[string]string{
		"alice": "1000000stake",
		"bob":   "1000000stake",
	}

	names := sortedNames(accounts)
	keys := make(map[string]cryptotypes.PrivKey)
	accNs := make(map[string]uint64)
	genAccounts := make([]simtestutil.GenesisAccount, 0, len(accounts))

	for i, name := range names {
		priv := secp256k1.GenPrivKey()
		keys[name] = priv
		accNs[name] = uint64(i)

		acc := authtypes.NewBaseAccount(
			priv.PubKey().Address().Bytes(),
			priv.PubKey(),
			uint64(i),
			0,
		)
		coins, err := sdk.ParseCoinsNormalized(accounts[name])
		require.NoError(t, err)
		genAccounts = append(genAccounts, simtestutil.GenesisAccount{
			GenesisAccount: acc,
			Coins:          coins,
		})
	}

	valSet, err := simtestutil.CreateRandomValidatorSet()
	require.NoError(t, err)

	baseCfg := simtestutil.StartupConfig{
		ValidatorSet:    func() (*cmttypes.ValidatorSet, error) { return valSet, nil },
		AtGenesis:       true,
		GenesisAccounts: genAccounts,
		DB:              dbm.NewMemDB(),
	}

	var txCfg client.TxConfig
	app, err := simtestutil.SetupWithConfiguration(testAppConfig(), baseCfg, &txCfg)
	require.NoError(t, err)

	return app, txCfg, testKeys{keys: keys, accNs: accNs}
}

func buildTx(t *testing.T, txCfg client.TxConfig, tk testKeys, signerName string, msg sdk.Msg) []byte {
	t.Helper()
	fromKey := tk.keys[signerName]
	accN := tk.accNs[signerName]

	tx, err := simtestutil.GenSignedMockTx(
		rand.New(rand.NewSource(42)),
		txCfg,
		[]sdk.Msg{msg},
		sdk.NewCoins(sdk.NewCoin("stake", sdkmath.NewInt(0))),
		200000,
		"",
		[]uint64{accN},
		[]uint64{0},
		fromKey,
	)
	require.NoError(t, err)

	txBytes, err := txCfg.TxEncoder()(tx)
	require.NoError(t, err)
	return txBytes
}

func TestCanaryTxsExecuteSuccessfully(t *testing.T) {
	app, txCfg, tk := setupApp(t)

	aliceKey := tk.keys["alice"]
	bobKey := tk.keys["bob"]

	tx1 := buildTx(t, txCfg, tk, "alice", &simcanarytypes.MsgCanaryMapSet{
		Sender: sdk.AccAddress(aliceKey.PubKey().Address()).String(),
		Key:    "k",
		Value:  42,
	})
	tx2 := buildTx(t, txCfg, tk, "bob", &simcanarytypes.MsgCanaryMapReadAndWrite{
		Sender: sdk.AccAddress(bobKey.PubKey().Address()).String(),
		Key:    "k",
	})

	res, err := app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height: 1,
		Txs:    [][]byte{tx1, tx2},
	})
	require.NoError(t, err)
	require.Len(t, res.TxResults, 2, "expected 2 tx results")

	for i, txRes := range res.TxResults {
		t.Logf("tx[%d] code=%d log=%s", i, txRes.Code, txRes.Log)
		require.Equalf(t, uint32(0), txRes.Code, "tx[%d] failed: %s", i, txRes.Log)
	}
}

func sortedNames(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

