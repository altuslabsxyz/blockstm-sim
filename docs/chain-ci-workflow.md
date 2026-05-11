# Chain-Side CI Integration — Workflow Example

This document shows how to wire `blockstm-sim` into a chain repository's CI.
The pattern applies to any Cosmos SDK chain built on a fork that has the
BlockSTM lifecycle hooks.

Replace `your-org/your-chain` and `chainapp` with your actual module path and
package name throughout.

---

## Directory layout in the chain repo

```
your-org/your-chain/
  integration/
    blockstm/
      sdkimpl.go        ← register sdkhook factories for your App
      factory.go        ← AppFactory using your NewApp()
      genesis.go        ← encode fixture GenesisSpec into chain genesis JSON
      blockstm_test.go  ← TestBlockSTM entry point
  .github/workflows/
    blockstm.yml        ← CI workflow
```

---

## `integration/blockstm/sdkimpl.go`

Registers the three sdkhook factories. This is the only file that imports
fork-specific packages (`txnrunner`, `lifecycle`). All other files use
`sdkhook` interfaces.

```go
//go:build sdk_hooks

package blockstm

import (
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp/lifecycle"
	"github.com/cosmos/cosmos-sdk/baseapp/txnrunner"
	sdk "github.com/cosmos/cosmos-sdk/types"

	chainapp "your-org/your-chain/app"
	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// chainAppAdapter adapts *chainapp.App to sdkhook.App.
// If your App is not *runtime.App directly, a chain-specific adapter is required.
type chainAppAdapter struct{ *chainapp.App }

// SetLifecycleObserver bridges compare.LifecycleObserver and the SDK fork's
// lifecycle.LifecycleObserver. The method sets are structurally identical.
func (a *chainAppAdapter) SetLifecycleObserver(obs compare.LifecycleObserver) {
	a.App.SetLifecycleObserver(obs.(lifecycle.LifecycleObserver))
}

// SetBlockSTMTxRunner bridges sdkhook.STMRunner (any) and sdk.TxRunner.
func (a *chainAppAdapter) SetBlockSTMTxRunner(runner sdkhook.STMRunner) {
	a.App.SetBlockSTMTxRunner(runner.(sdk.TxRunner))
}

func init() {
	// 1. Keeper discovery — enumerate all module instances for the
	//    reflective out-of-KV tracker.
	sdkhook.RegisterKeeperDiscovery(func(raw any) []any {
		app := raw.(*chainapp.App)
		mods := make([]any, 0, len(app.ModuleManager.Modules))
		for _, mod := range app.ModuleManager.Modules {
			mods = append(mods, mod)
		}
		return mods
	})

	// 2. App wrapper — produce an sdkhook.App from a raw *chainapp.App.
	sdkhook.RegisterAppWrapper(func(rawApp any) sdkhook.App {
		return &chainAppAdapter{rawApp.(*chainapp.App)}
	})

	// 3. STM runner factory — construct the BlockSTM parallel runner.
	//    perturbSeed == 0: oracle / baseline probe (no perturbation).
	//    perturbSeed != 0: probe mode with scheduler race-widening.
	sdkhook.RegisterSTMRunnerFactory(func(
		decoder sdk.TxDecoder,
		keys []storetypes.StoreKey,
		workers int,
		perturbSeed int64,
	) sdkhook.STMRunner {
		if perturbSeed == 0 {
			return txnrunner.NewSTMRunner(decoder, keys, workers, false, nil)
		}
		return txnrunner.NewSTMRunner(decoder, keys, workers, false, nil,
			txnrunner.WithPerturbHook(perturbSeed))
	})
}
```

---

## `integration/blockstm/factory.go`

Constructs a fresh chain app from a blockstm-sim `GenesisSpec` and returns
it as an `sdkhook.App`.

```go
//go:build sdk_hooks

package blockstm

import (
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"cosmossdk.io/log"

	chainapp "your-org/your-chain/app"
	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/run"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// ChainAppFactory returns an AppFactory that creates a chain app instance
// initialised from a blockstm-sim GenesisSpec.
func ChainAppFactory(genesis compare.GenesisSpec) run.AppFactory {
	return func(db dbm.DB, txCfgOut *client.TxConfig, _ ...any) (sdkhook.App, any, error) {
		// MakeEncodingConfig is chain-specific; adjust as needed.
		encodingConfig := chainapp.MakeEncodingConfig()

		if txCfgOut != nil {
			*txCfgOut = encodingConfig.TxConfig
		}

		appOpts := simtestutil.NewAppOptionsWithFlagHome(chainapp.DefaultNodeHome)
		app := chainapp.NewApp(
			log.NewNopLogger(),
			db,
			nil,  // traceStore
			true, // loadLatest
			appOpts.(servertypes.AppOptions),
			// add any chain-specific constructor arguments here
		)

		initChainReq, err := buildGenesisFromSpec(genesis, app, encodingConfig)
		if err != nil {
			return nil, nil, err
		}
		if _, err := app.InitChain(initChainReq); err != nil {
			return nil, nil, err
		}

		return sdkhook.WrapApp(app), app, nil
	}
}

// buildGenesisFromSpec encodes a blockstm-sim GenesisSpec into the chain's
// genesis JSON format. Accounts and balances from the spec are written into
// the auth and bank modules; all other modules use their default genesis.
//
// Reference implementation: run/executor.go:initGenesis in blockstm-sim
// (simtestutil-based). Adapt to your chain's genesis encoding as needed.
func buildGenesisFromSpec(
	genesis compare.GenesisSpec,
	app *chainapp.App,
	encodingConfig chainapp.EncodingConfig,
) (*abci.RequestInitChain, error) {
	defaultGenesis := app.DefaultGenesis()

	// --- auth module ---
	// Populate authtypes.GenesisState with accounts derived from
	// genesis.Accounts. Use run.DeriveKey(name) for deterministic keys.

	// --- bank module ---
	// Populate banktypes.GenesisState with balances from genesis.Accounts.

	genStateBytes, err := json.Marshal(defaultGenesis)
	if err != nil {
		return nil, err
	}

	valSet, _ := simtestutil.CreateRandomValidatorSet()
	return &abci.RequestInitChain{
		ChainId:       "blockstm-test-1", // use a dedicated test chain ID
		AppStateBytes: genStateBytes,
		InitialHeight: 1,
		// encode valSet into Validators field
		_ = valSet
	}, nil
}
```

> **Note:** Fill in the auth/bank genesis encoding in `buildGenesisFromSpec`.
> See `run/executor.go:initGenesis` in blockstm-sim for the reference pattern.

---

## `integration/blockstm/blockstm_test.go`

Test entry point that wires the factory and runs the harness.

```go
//go:build sdk_hooks

package blockstm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/report"
	"github.com/altuslabsxyz/blockstm-sim/run"
)

func TestMain(m *testing.M) {
	// init() in sdkimpl.go registers the sdkhook factories.
	os.Exit(m.Run())
}

func TestBlockSTM_FixtureCorpus(t *testing.T) {
	corpusDir := os.Getenv("BLOCKSTM_CORPUS")
	if corpusDir == "" {
		t.Fatal("BLOCKSTM_CORPUS env var required: path to blockstm-sim/corpus/fixtures")
	}

	stores, err := compare.LoadCorpusStores(corpusDir)
	require.NoError(t, err)
	require.NotEmpty(t, stores, "corpus must contain at least one fixture")

	// All fixtures share the same genesis accounts.
	genesis := stores[0].Genesis()

	executor := run.NewFixtureExecutor(run.WithAppFactory(ChainAppFactory(genesis)))

	rep := report.NewCLI(os.Stdout, os.Stderr)
	cfg := run.Config{
		CorpusDir:        corpusDir,
		Probes:           1,
		FailOnDivergence: true,
	}

	code := run.RunHarness(cfg, executor, stores, rep, os.Stderr)
	require.Zero(t, code, "blockstm divergence or missed canary detected")
}
```

---

## `.github/workflows/blockstm.yml`

Runs on every PR that touches the execution path (`app/`, `x/`).

```yaml
name: blockstm-safety

on:
  pull_request:
    paths:
      - "app/**"
      - "x/**"
      - "go.mod"

permissions:
  contents: read

jobs:
  simulate:
    runs-on: ubuntu-latest
    env:
      BLOCKSTM_CORPUS_DIR: /tmp/blockstm-corpus

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.25.x"

      - name: Fetch blockstm-sim corpus
        run: |
          git clone --depth=1 \
            https://github.com/altuslabsxyz/blockstm-sim.git /tmp/blockstm-sim
          cp -r /tmp/blockstm-sim/corpus/fixtures "${BLOCKSTM_CORPUS_DIR}"

      - name: Run blockstm-sim integration tests
        env:
          BLOCKSTM_CORPUS: ${{ env.BLOCKSTM_CORPUS_DIR }}
        run: |
          go test \
            -tags "sdk_hooks simharness simharness_canary" \
            -timeout 10m \
            -v \
            ./integration/blockstm/...

      - name: Upload simulation report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: blockstm-report
          path: "*.md"
          if-no-files-found: ignore
```

---

## Prerequisites

The SDK fork must expose the following on `*App` before this workflow runs:

- `SetLifecycleObserver(LifecycleObserver)` — wires the block/tx lifecycle observer
- `UnsetBlockSTMTxRunner()` — reverts to sequential execution (oracle mode)
- `SetBlockSTMTxRunner(TxRunner)` — installs the parallel runner (probe mode)
- `SetDisableBlockGasMeter(bool)` — disables gas metering for simulation runs
- `GetStoreKeys() []StoreKey` — enumerates all multistore keys

The fixture corpus test (F1 comparison only) works with the above.
Snapshot replay additionally requires `CacheMultiStoreWithVersion` on the
multistore, and scheduler perturbation requires a `PerturbHook` on the
BlockSTM scheduler.

`buildGenesisFromSpec` must be implemented before the test passes with real
bank-send transactions.
