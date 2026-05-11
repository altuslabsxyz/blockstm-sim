# Chain-Side CI Integration — Workflow Example

This document shows how to wire `blockstm-sim` into a chain repository's CI.
The target is `stablelabs/stable`, but the pattern applies to any Cosmos SDK
chain that runs on `stablelabs/stable-sdk`.

---

## Directory layout in the chain repo

```
stablelabs/stable/
  integration/
    blockstm/
      sdkimpl.go        ← register sdkhook factories for stable.App
      factory.go        ← AppFactory using stable.NewApp()
      genesis.go        ← encode fixture GenesisSpec into stable genesis JSON
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

	stableapp "github.com/stablelabs/stable/app"
	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// stableAppAdapter adapts *stableapp.App to sdkhook.App.
// stable.App is not *runtime.App, so a chain-specific adapter is required.
type stableAppAdapter struct{ *stableapp.App }

// SetLifecycleObserver bridges compare.LifecycleObserver and the SDK fork's
// lifecycle.LifecycleObserver. The method sets are structurally identical.
func (a *stableAppAdapter) SetLifecycleObserver(obs compare.LifecycleObserver) {
	a.App.SetLifecycleObserver(obs.(lifecycle.LifecycleObserver))
}

// SetBlockSTMTxRunner bridges sdkhook.STMRunner (any) and sdk.TxRunner.
func (a *stableAppAdapter) SetBlockSTMTxRunner(runner sdkhook.STMRunner) {
	a.App.SetBlockSTMTxRunner(runner.(sdk.TxRunner))
}

func init() {
	// 1. Keeper discovery — enumerate all module instances for the
	//    reflective out-of-KV tracker.
	sdkhook.RegisterKeeperDiscovery(func(raw any) []any {
		app := raw.(*stableapp.App)
		mods := make([]any, 0, len(app.ModuleManager.Modules))
		for _, mod := range app.ModuleManager.Modules {
			mods = append(mods, mod)
		}
		return mods
	})

	// 2. App wrapper — produce an sdkhook.App from a raw *stableapp.App.
	sdkhook.RegisterAppWrapper(func(rawApp any) sdkhook.App {
		return &stableAppAdapter{rawApp.(*stableapp.App)}
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

Constructs a fresh `stable.App` from a blockstm-sim `GenesisSpec` and returns
it as an `sdkhook.App`.

```go
//go:build sdk_hooks

package blockstm

import (
	"encoding/json"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"

	stableapp "github.com/stablelabs/stable/app"
	appcfg "github.com/stablelabs/stable/app/config"
	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/run"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"

	"cosmossdk.io/log"
)

// StableAppFactory returns an AppFactory that creates a stable.App instance
// initialised from a blockstm-sim GenesisSpec.
func StableAppFactory(genesis compare.GenesisSpec) run.AppFactory {
	return func(db dbm.DB, txCfgOut *client.TxConfig, _ ...any) (sdkhook.App, any, error) {
		evmChainID := appcfg.GetEVMChainID()
		encodingConfig := stableapp.MakeEncodingConfig(evmChainID)

		if txCfgOut != nil {
			*txCfgOut = encodingConfig.TxConfig
		}

		appOpts := simtestutil.NewAppOptionsWithFlagHome(stableapp.DefaultNodeHome)
		app := stableapp.NewApp(
			log.NewNopLogger(),
			db,
			nil,   // traceStore
			true,  // loadLatest
			appOpts.(servertypes.AppOptions),
			evmChainID,
		)

		genBytes, err := buildGenesisFromSpec(genesis, app, encodingConfig)
		if err != nil {
			return nil, nil, err
		}

		if _, err := app.InitChain(genBytes); err != nil {
			return nil, nil, err
		}

		return sdkhook.WrapApp(app), app, nil
	}
}

// buildGenesisFromSpec encodes a blockstm-sim GenesisSpec into stable's
// genesis JSON format. Accounts and balances from the spec are written into
// the auth and bank modules; all other modules use their default genesis.
func buildGenesisFromSpec(
	genesis compare.GenesisSpec,
	app *stableapp.App,
	encodingConfig stableapp.EncodingConfig,
) (*abci.RequestInitChain, error) {
	defaultGenesis := app.DefaultGenesis()

	// --- auth module ---
	// ... populate authtypes.GenesisState with accounts from genesis.Accounts
	// See: https://github.com/altuslabsxyz/blockstm-sim/blob/main/run/executor.go
	// for the simtestutil-based reference implementation.

	// --- bank module ---
	// ... populate banktypes.GenesisState with balances from genesis.Accounts

	genStateBytes, err := json.Marshal(defaultGenesis)
	if err != nil {
		return nil, err
	}

	valSet, _ := simtestutil.CreateRandomValidatorSet()
	return &abci.RequestInitChain{
		ChainId:         appcfg.GetChainID(),
		AppStateBytes:   genStateBytes,
		InitialHeight:   1,
		// validators from valSet ...
	}, nil
}
```

> **Note:** `buildGenesisFromSpec` needs to be filled in with the auth/bank
> genesis encoding. See `run/executor.go:initGenesis` in blockstm-sim for the
> reference implementation using simtestutil types.

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

	_ "github.com/altuslabsxyz/blockstm-sim/cmd/blockstm-sim/sdkimpl" // NOT used here
	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/report"
	"github.com/altuslabsxyz/blockstm-sim/run"
)

func TestMain(m *testing.M) {
	// sdkimpl init() in this package registers the sdkhook factories.
	os.Exit(m.Run())
}

func TestBlockSTM_FixtureCorpus(t *testing.T) {
	corpusDir := os.Getenv("BLOCKSTM_CORPUS")
	if corpusDir == "" {
		corpusDir = "../../vendor/blockstm-corpus/fixtures"
	}

	stores, err := compare.LoadCorpusStores(corpusDir)
	require.NoError(t, err)
	require.NotEmpty(t, stores, "corpus must contain at least one fixture")

	// Derive genesis from the first fixture (all fixtures share the same accounts).
	genesis := stores[0].Genesis()

	executor := run.NewFixtureExecutor(run.WithAppFactory(StableAppFactory(genesis)))

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
      BLOCKSTM_SIM_REF: main   # pin to a release tag in production
      BLOCKSTM_CORPUS_DIR: /tmp/blockstm-corpus

    steps:
      - name: Checkout stable
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.25.x"

      - name: Checkout blockstm-sim corpus
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

## Prerequisites checklist

Before this workflow can run end-to-end:

| Item | Status | Issue |
|---|---|---|
| `LifecycleObserver` in stable-sdk | In Review | PLA-47 |
| Hook fire points in BaseApp | In Review | PLA-48 |
| `SetLifecycleObserver` / `UnsetBlockSTMTxRunner` | In Review | PLA-49 |
| `CacheMultiStoreWithVersion` (snapshot corpus) | In Review | PLA-54 |
| `PerturbHook` (F2 repeat-determinism) | In Review | PLA-55 |
| `buildGenesisFromSpec` implementation | — | PLA-81 |

The fixture corpus test (F1 comparison only, no snapshot) works once
PLA-47/48/49 are merged. PLA-54/55 are required only for snapshot replay
and scheduler perturbation.
