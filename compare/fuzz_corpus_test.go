package compare_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/coverage"
)

func withFuzzRegistry(t *testing.T, keys ...string) {
	t.Helper()
	coverage.ClearRegistry()
	for _, k := range keys {
		coverage.Register(k, coverage.Entry{Key: k, Module: "test", MsgType: "Msg" + k, HandlerFn: k})
	}
	t.Cleanup(coverage.ClearRegistry)
}

func TestFuzzCorpus_SeedReproducibility(t *testing.T) {
	withFuzzRegistry(t, "bank-send", "bank-multi-send")

	cfg := compare.FuzzConfig{
		Seed:       42,
		Blocks:     10,
		TxPerBlock: 3,
	}

	collect := func() []compare.Block {
		fc := compare.NewFuzzCorpus(cfg)
		var blocks []compare.Block
		for block, err := range fc.Iter(context.Background()) {
			require.NoError(t, err)
			blocks = append(blocks, block)
		}
		return blocks
	}

	first := collect()
	second := collect()

	require.Equal(t, first, second, "same seed must produce identical block sequences")
	require.Len(t, first, 10)
	for _, b := range first {
		require.Len(t, b.Txs, 3)
	}
}

func TestFuzzCorpus_DifferentSeedsDiffer(t *testing.T) {
	withFuzzRegistry(t, "bank-send", "bank-multi-send")

	collect := func(seed int64) []compare.Block {
		cfg := compare.FuzzConfig{Seed: seed, Blocks: 5, TxPerBlock: 2}
		fc := compare.NewFuzzCorpus(cfg)
		var blocks []compare.Block
		for block, err := range fc.Iter(context.Background()) {
			require.NoError(t, err)
			blocks = append(blocks, block)
		}
		return blocks
	}

	require.NotEqual(t, collect(1), collect(2))
}

func TestFuzzCorpus_WeightedSampling(t *testing.T) {
	withFuzzRegistry(t, "bank-send", "bank-multi-send")

	// Weight bank-send 100x more than bank-multi-send.
	cfg := compare.FuzzConfig{
		Seed:       99,
		Blocks:     20,
		TxPerBlock: 10,
		Weights:    map[string]int{"bank-send": 100, "bank-multi-send": 1},
	}
	fc := compare.NewFuzzCorpus(cfg)

	counts := map[string]int{}
	for block, err := range fc.Iter(context.Background()) {
		require.NoError(t, err)
		for _, tx := range block.Txs {
			counts[tx.Msg]++
		}
	}

	require.Greater(t, counts["bank-send"], counts["bank-multi-send"],
		"heavily weighted key should appear more often")
}

func TestFuzzCorpus_UnregisteredWeightKeySkipped(t *testing.T) {
	withFuzzRegistry(t, "bank-send")

	// "unknown" is in weights but not registered — should be silently skipped.
	cfg := compare.FuzzConfig{
		Seed:       1,
		Blocks:     5,
		TxPerBlock: 3,
		Weights:    map[string]int{"bank-send": 1, "unknown": 100},
	}
	fc := compare.NewFuzzCorpus(cfg)

	for block, err := range fc.Iter(context.Background()) {
		require.NoError(t, err)
		for _, tx := range block.Txs {
			require.Equal(t, "bank-send", tx.Msg, "only registered keys should appear")
		}
	}
}

func TestFuzzCorpus_DefaultGenesisAccounts(t *testing.T) {
	withFuzzRegistry(t, "bank-send")

	// No genesis in config — defaults to alice, bob, charlie.
	cfg := compare.FuzzConfig{Seed: 1, Blocks: 3, TxPerBlock: 1}
	fc := compare.NewFuzzCorpus(cfg)

	validAccounts := map[string]bool{"alice": true, "bob": true, "charlie": true}
	for block, err := range fc.Iter(context.Background()) {
		require.NoError(t, err)
		for _, tx := range block.Txs {
			require.True(t, validAccounts[tx.Signer], "signer %q not in default genesis", tx.Signer)
			require.True(t, validAccounts[tx.To], "recipient %q not in default genesis", tx.To)
		}
	}
}

func TestFuzzCorpus_IterCancel(t *testing.T) {
	withFuzzRegistry(t, "bank-send")

	cfg := compare.FuzzConfig{Seed: 1, Blocks: 100, TxPerBlock: 1}
	fc := compare.NewFuzzCorpus(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		blocks  []compare.Block
		iterErr error
	)
	for block, err := range fc.Iter(ctx) {
		if err != nil {
			iterErr = err
			break
		}
		blocks = append(blocks, block)
		if len(blocks) == 3 {
			cancel()
		}
	}

	require.ErrorIs(t, iterErr, context.Canceled)
	require.Len(t, blocks, 3)
}

func TestFuzzCorpus_CorpusStoreMethods(t *testing.T) {
	withFuzzRegistry(t, "bank-send")

	cfg := compare.FuzzConfig{Seed: 7, Blocks: 5, TxPerBlock: 2}
	fc := compare.NewFuzzCorpus(cfg)

	require.Equal(t, "fuzz", fc.Name())
	require.Equal(t, "stake", fc.BondDenom())
	require.Equal(t, 5, fc.BlockCount())
	require.False(t, fc.IsCanary())
	require.Nil(t, fc.PreStateDB())
	require.NoError(t, fc.Close())
}

func TestLoadCorpusStores_FuzzJSON(t *testing.T) {
	withFuzzRegistry(t, "bank-send")

	dir := t.TempDir()
	cfg := compare.FuzzConfig{
		Seed:       42,
		Blocks:     10,
		TxPerBlock: 3,
		Weights:    map[string]int{"bank-send": 1},
		Genesis: compare.GenesisSpec{
			Accounts: map[string]compare.AccountSpec{
				"alice": {Balance: "1000000stake"},
				"bob":   {Balance: "1000000stake"},
			},
		},
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fuzz.json"), data, 0o644))

	stores, err := compare.LoadCorpusStores(dir)
	require.NoError(t, err)
	require.Len(t, stores, 1)

	fc, ok := stores[0].(*compare.FuzzCorpus)
	require.True(t, ok, "expected *compare.FuzzCorpus, got %T", stores[0])
	require.Equal(t, "fuzz", fc.Name())
	require.Equal(t, 10, fc.BlockCount())
}

func TestLoadCorpusStores_YAMLFallback(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: fallback
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fixture.yaml"), []byte(yaml), 0o644))

	stores, err := compare.LoadCorpusStores(dir)
	require.NoError(t, err)
	require.Len(t, stores, 1)

	_, ok := stores[0].(*compare.FixtureCorpus)
	require.True(t, ok, "expected *compare.FixtureCorpus when no fuzz.json")
}
