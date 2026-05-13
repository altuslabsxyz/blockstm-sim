package compare_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestFixtureCorpus_Iter(t *testing.T) {
	fixture := compare.Fixture{
		Name: "iter-test",
		Genesis: compare.GenesisSpec{
			Accounts: map[string]compare.AccountSpec{
				"alice": {Balance: "1000000stake"},
			},
		},
		Blocks: []compare.BlockSpec{
			{Txs: []compare.TxSpec{{Signer: "alice", Msg: "bank-send", To: "alice", Amount: "100stake", Gas: 200000}}},
			{Txs: []compare.TxSpec{{Signer: "alice", Msg: "bank-send", To: "alice", Amount: "200stake", Gas: 200000}}},
			{Txs: []compare.TxSpec{{Signer: "alice", Msg: "bank-send", To: "alice", Amount: "300stake", Gas: 200000}}},
		},
	}

	fc := compare.NewFixtureCorpus(fixture)
	var blocks []compare.Block
	for block, err := range fc.Iter(context.Background()) {
		require.NoError(t, err)
		blocks = append(blocks, block)
	}

	require.Len(t, blocks, 3)
	require.Equal(t, "100stake", blocks[0].Txs[0].Amount)
	require.Equal(t, "200stake", blocks[1].Txs[0].Amount)
	require.Equal(t, "300stake", blocks[2].Txs[0].Amount)
}

func TestFixtureCorpus_IterCancel(t *testing.T) {
	fixture := compare.Fixture{
		Name: "cancel-test",
		Genesis: compare.GenesisSpec{
			Accounts: map[string]compare.AccountSpec{
				"alice": {Balance: "1000000stake"},
			},
		},
		Blocks: []compare.BlockSpec{
			{Txs: []compare.TxSpec{{Signer: "alice", Msg: "bank-send", To: "alice", Amount: "100stake", Gas: 200000}}},
			{Txs: []compare.TxSpec{{Signer: "alice", Msg: "bank-send", To: "alice", Amount: "200stake", Gas: 200000}}},
			{Txs: []compare.TxSpec{{Signer: "alice", Msg: "bank-send", To: "alice", Amount: "300stake", Gas: 200000}}},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fc := compare.NewFixtureCorpus(fixture)

	var (
		blocks []compare.Block
		iterErr error
	)
	for block, err := range fc.Iter(ctx) {
		if err != nil {
			iterErr = err
			break
		}
		blocks = append(blocks, block)
		// Cancel after the first block.
		if len(blocks) == 1 {
			cancel()
		}
	}

	require.Error(t, iterErr)
	require.ErrorIs(t, iterErr, context.Canceled)
	require.Len(t, blocks, 1)
}

func TestLoadCorpusStores(t *testing.T) {
	dir := t.TempDir()

	fixture1 := `name: alpha-fixture
genesis:
  accounts:
    alice:
      balance: "1000000stake"
blocks:
  - txs:
      - signer: alice
        msg: bank-send
        to: alice
        amount: "100stake"
        gas: 200000
`
	fixture2 := `name: beta-fixture
genesis:
  accounts:
    bob:
      balance: "2000000stake"
blocks:
  - txs:
      - signer: bob
        msg: bank-send
        to: bob
        amount: "500stake"
        gas: 200000
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alpha.yaml"), []byte(fixture1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "beta.yaml"), []byte(fixture2), 0o644))

	stores, err := compare.LoadCorpusStores(dir)
	require.NoError(t, err)
	require.Len(t, stores, 2)

	// LoadCorpus sorts by name, so alpha comes first.
	fc0, ok := stores[0].(*compare.FixtureCorpus)
	require.True(t, ok)
	require.Equal(t, "alpha-fixture", fc0.Name())

	fc1, ok := stores[1].(*compare.FixtureCorpus)
	require.True(t, ok)
	require.Equal(t, "beta-fixture", fc1.Name())
}

func TestFixtureCorpus_Methods(t *testing.T) {
	fixture := compare.Fixture{
		Name: "methods-test",
		Kind: compare.KindCanary,
		Genesis: compare.GenesisSpec{
			Accounts: map[string]compare.AccountSpec{
				"alice": {Balance: "5000000stake"},
			},
		},
		Blocks: []compare.BlockSpec{
			{Txs: []compare.TxSpec{{Signer: "alice", Msg: "bank-send", To: "alice", Amount: "100stake", Gas: 200000}}},
		},
	}

	fc := compare.NewFixtureCorpus(fixture)

	require.Equal(t, "methods-test", fc.Name())
	require.True(t, fc.IsCanary())
	require.Equal(t, "stake", fc.BondDenom())
	require.Nil(t, fc.PreStateDB())
	require.NoError(t, fc.Close())

	genesis := fc.Genesis()
	require.Contains(t, genesis.Accounts, "alice")
	require.Equal(t, "5000000stake", genesis.Accounts["alice"].Balance)
}

func TestLoadCorpusStores_RepoFuzzCorpus(t *testing.T) {
	// Verify that the committed corpus/fuzz/fuzz.json loads as a FuzzCorpus.
	stores, err := compare.LoadCorpusStores("../corpus/fuzz")
	require.NoError(t, err)
	require.Len(t, stores, 1)

	fc, ok := stores[0].(*compare.FuzzCorpus)
	require.True(t, ok, "expected *compare.FuzzCorpus, got %T", stores[0])
	require.Equal(t, 50, fc.BlockCount())
	require.Equal(t, "fuzz", fc.Name())
}

func TestBlockSpec_RawTxs(t *testing.T) {
	b := compare.BlockSpec{
		Height: 12345678,
		RawTxs: [][]byte{[]byte("tx1"), []byte("tx2")},
	}
	require.Equal(t, int64(12345678), b.Height)
	require.Len(t, b.RawTxs, 2)
	require.Equal(t, []byte("tx1"), b.RawTxs[0])
}
