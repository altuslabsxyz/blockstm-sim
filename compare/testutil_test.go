//go:build sdk_hooks

package compare_test

import (
	"crypto/sha256"
	"math/rand"
	"sort"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

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
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	_ "github.com/cosmos/cosmos-sdk/x/consensus"
	_ "github.com/cosmos/cosmos-sdk/x/distribution"
	_ "github.com/cosmos/cosmos-sdk/x/mint"
	_ "github.com/cosmos/cosmos-sdk/x/protocolpool"
	_ "github.com/cosmos/cosmos-sdk/x/staking"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/instrument"
)

// ---------------------------------------------------------------------------
// Minimal app config (auth + bank + supporting modules)
// ---------------------------------------------------------------------------

var testAppConfig = depinject.Configs(
	configurator.NewAppConfig(
		configurator.AuthModule(),
		configurator.BankModule(),
		configurator.StakingModule(),
		configurator.MintModule(),
		configurator.DistributionModule(),
		configurator.ProtocolPoolModule(),
		configurator.ConsensusModule(),
		configurator.TxModule(),
	),
	depinject.Supply(log.NewNopLogger()),
)

// ---------------------------------------------------------------------------
// App pair (oracle + probe)
// ---------------------------------------------------------------------------

type appPair struct {
	Oracle   *runtime.App
	Probe    *runtime.App
	TxConfig client.TxConfig
	Keys     map[string]cryptotypes.PrivKey
}

func newAppPair(t *testing.T, accounts map[string]compare.AccountSpec) appPair {
	t.Helper()

	names := sortedKeys(accounts)

	keys := make(map[string]cryptotypes.PrivKey, len(accounts))
	genAccounts := make([]simtestutil.GenesisAccount, 0, len(accounts))
	for i, name := range names {
		seed := sha256.Sum256([]byte("blockstm-sim:" + name))
		priv := &secp256k1.PrivKey{Key: seed[:]}
		keys[name] = priv

		acc := authtypes.NewBaseAccount(
			priv.PubKey().Address().Bytes(),
			priv.PubKey(),
			uint64(i),
			0,
		)
		coins, err := sdk.ParseCoinsNormalized(accounts[name].Balance)
		require.NoError(t, err)

		genAccounts = append(genAccounts, simtestutil.GenesisAccount{
			GenesisAccount: acc,
			Coins:          coins,
		})
	}

	valSet, err := simtestutil.CreateRandomValidatorSet()
	require.NoError(t, err)
	valSetFn := func() (*cmttypes.ValidatorSet, error) { return valSet, nil }

	baseCfg := simtestutil.StartupConfig{
		ValidatorSet:    valSetFn,
		AtGenesis:       true,
		GenesisAccounts: genAccounts,
	}

	var txCfg client.TxConfig

	baseCfg.DB = dbm.NewMemDB()
	oracleApp, err := simtestutil.SetupWithConfiguration(testAppConfig, baseCfg, &txCfg)
	require.NoError(t, err)
	instrument.InstrumentApp(oracleApp, instrument.Options{Runner: instrument.RunnerSequential})

	baseCfg.DB = dbm.NewMemDB()
	probeApp, err := simtestutil.SetupWithConfiguration(testAppConfig, baseCfg)
	require.NoError(t, err)

	return appPair{
		Oracle:   oracleApp,
		Probe:    probeApp,
		TxConfig: txCfg,
		Keys:     keys,
	}
}

// ---------------------------------------------------------------------------
// Transaction builder
// ---------------------------------------------------------------------------

func buildBankSendTx(t *testing.T, pair appPair, spec compare.TxSpec) []byte {
	t.Helper()

	fromKey := pair.Keys[spec.Signer]
	toKey := pair.Keys[spec.To]

	amount, err := sdk.ParseCoinsNormalized(spec.Amount)
	require.NoError(t, err)

	msg := banktypes.NewMsgSend(
		fromKey.PubKey().Address().Bytes(),
		toKey.PubKey().Address().Bytes(),
		amount,
	)

	tx, err := simtestutil.GenSignedMockTx(
		rand.New(rand.NewSource(42)),
		pair.TxConfig,
		[]sdk.Msg{msg},
		sdk.NewCoins(sdk.NewCoin("stake", sdkmath.NewInt(0))),
		spec.Gas,
		"",          // chain ID (empty, matches default InitChain)
		[]uint64{0}, // account number
		[]uint64{0}, // sequence
		fromKey,
	)
	require.NoError(t, err)

	txBytes, err := pair.TxConfig.TxEncoder()(tx)
	require.NoError(t, err)
	return txBytes
}

// ---------------------------------------------------------------------------
// Divergent finalizer (corrupts app hash)
// ---------------------------------------------------------------------------

type divergentFinalizer struct {
	compare.Finalizer
}

func (d *divergentFinalizer) FinalizeBlock(req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	res, err := d.Finalizer.FinalizeBlock(req)
	if err != nil {
		return res, err
	}
	corrupted := make([]byte, len(res.AppHash))
	copy(corrupted, res.AppHash)
	corrupted[0] ^= 0xFF
	res.AppHash = corrupted
	return res, nil
}

// ---------------------------------------------------------------------------
// Error code finalizer (injects tx results with given codes)
// ---------------------------------------------------------------------------

type errorCodeFinalizer struct {
	compare.Finalizer
	codes []uint32
}

func (f *errorCodeFinalizer) FinalizeBlock(req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	res, err := f.Finalizer.FinalizeBlock(req)
	if err != nil {
		return res, err
	}
	txResults := make([]*abci.ExecTxResult, len(f.codes))
	for i, code := range f.codes {
		txResults[i] = &abci.ExecTxResult{Code: code}
	}
	res.TxResults = txResults
	return res, nil
}

// ---------------------------------------------------------------------------
// Mock write set provider
// ---------------------------------------------------------------------------

type mockWriteSetProvider struct {
	sets map[int][]string
}

func (m *mockWriteSetProvider) TxWriteSet(txIndex int) []string {
	return m.sets[txIndex]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sortedKeys(m map[string]compare.AccountSpec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---------------------------------------------------------------------------
// Mock mutation provider
// ---------------------------------------------------------------------------

type mockMutationProvider struct {
	muts map[int][]compare.MutationRecord
}

func (m *mockMutationProvider) TxMutations(txIndex int) []compare.MutationRecord {
	return m.muts[txIndex]
}
