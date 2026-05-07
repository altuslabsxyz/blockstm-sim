# SDK Fork-Specific API Inventory

> **Purpose**: Identify every call site in `blockstm-sim` that depends on a cosmos-sdk fork's
> BlockSTM-specific additions. This drives the interface design in `sdkhook/sdkhook.go`.
>
> **Rule**: Anything in this doc's "Must be interfaced" section cannot remain as a direct import
> in `run/` or `instrument/`. It must be hidden behind an interface defined in `sdkhook/`.

---

## Summary

| Category | Count | Action |
|----------|-------|--------|
| App lifecycle methods (fork-added) | 5 methods | → `sdkhook.App` interface |
| STM runner construction | 2 funcs + 1 type | → `sdkhook.STMRunner` + `STMRunnerFactory` |
| `sdk.TxRunner` type | 1 type | → `sdkhook.STMRunner` (absorbs) |
| App constructor (`simtestutil`) | 1 func + 3 types | → `sdkhook.AppFactory` |
| Upstream types | many | stay as direct imports |

---

## Must Be Interfaced (fork-specific)

### 1. App lifecycle methods

Methods added to `runtime.App` in the SDK fork. Not in upstream cosmos-sdk.

| Method | Signature | Call sites |
|--------|-----------|-----------|
| `SetLifecycleObserver` | `(compare.LifecycleObserver)` | `run/executor.go:273,274,304,305`<br>`run/repeat_executor.go:105,111,171,173` |
| `UnsetBlockSTMTxRunner` | `()` | `instrument/instrument.go:35`<br>`run/repeat_executor.go:57` |
| `SetBlockSTMTxRunner` | `(sdk.TxRunner)` | `instrument/instrument_stm.go:15` |
| `SetDisableBlockGasMeter` | `(bool)` | `instrument/instrument_stm.go:14` |
| `GetStoreKeys` | `() []storetypes.StoreKey` | `run/executor.go:217`<br>`run/repeat_executor.go:68` |

**→ Becomes**: `sdkhook.App` interface. `*runtime.App` fields in `FixtureExecutor` and `RepeatRunExecutor` become `sdkhook.App`.

---

### 2. STM runner construction

`github.com/cosmos/cosmos-sdk/baseapp/txnrunner` — fork-only package, does not exist in upstream.

| Symbol | Signature | Call sites |
|--------|-----------|-----------|
| `txnrunner.NewSTMRunner` | `(sdk.TxDecoder, []storetypes.StoreKey, int, bool, any, ...Option) *STMRunner` | `run/executor.go:217,228`<br>`run/perturb.go:16,18` |
| `txnrunner.WithPerturbHook` | `(seed int64) Option` | `run/executor.go:228`<br>`run/perturb.go:19` |
| `*txnrunner.STMRunner` | return type | `run/perturb.go:14` (param + return) |

**→ Becomes**: `sdkhook.STMRunner` interface + `sdkhook.STMRunnerFactory` function type:
```go
type STMRunnerFactory func(decoder sdk.TxDecoder, keys []storetypes.StoreKey, workers int, perturbSeed int64) STMRunner
```
`WithPerturbHook` folds into the factory (`perturbSeed == 0` means no perturbation).

---

### 3. `sdk.TxRunner` type

`github.com/cosmos/cosmos-sdk/types.TxRunner` — interface type defined in the SDK fork.
Used as the argument to `SetBlockSTMTxRunner`.

| Symbol | Signature | Call sites |
|--------|-----------|-----------|
| `sdk.TxRunner` | interface | `instrument/instrument_stm.go:9,13` |

**→ Absorbed into `sdkhook.STMRunner`**. `InstrumentSTM` signature becomes:
```go
func InstrumentSTM(app sdkhook.App, runner sdkhook.STMRunner)
```

---

### 4. App constructor (`simtestutil`)

`github.com/cosmos/cosmos-sdk/testutil/sims.SetupWithConfiguration` — the only path to
constructing a `*runtime.App`. Must be replaced by a registered `AppFactory` so that different
chains can supply their own constructor.

| Symbol | Used in | Action |
|--------|---------|--------|
| `simtestutil.SetupWithConfiguration` | `run/executor.go:48,54,208`<br>`run/repeat_executor.go:53,64` | → `sdkhook.AppFactory` |
| `simtestutil.StartupConfig` | `run/executor.go:352,391-395` | stays in canary adapter |
| `simtestutil.GenesisAccount` | `run/executor.go:364,381` | stays in canary adapter |
| `simtestutil.CreateRandomValidatorSet` | `run/executor.go:387` | stays in canary adapter |
| `simtestutil.GenSignedMockTx` | `run/executor.go:108` | stays in canary adapter |

`StartupConfig`, `GenesisAccount`, `CreateRandomValidatorSet`, `GenSignedMockTx` remain inside
the chain-specific adapter code (canary self-test). They are not called from chain-agnostic core.

**→ Becomes**: `sdkhook.AppFactory`:
```go
type AppFactory func(genesis compare.GenesisSpec) (App, client.TxConfig, error)
```

---

## Stays as Direct Import (upstream)

These are in unmodified upstream cosmos-sdk or cometbft. No abstraction needed.

| Package | Symbols used | Notes |
|---------|-------------|-------|
| `github.com/cometbft/cometbft/abci/types` | `RequestFinalizeBlock`, `ResponseFinalizeBlock` | upstream |
| `cosmossdk.io/store/types` | `StoreKey` | upstream |
| `github.com/cosmos/cosmos-sdk/types` | `TxDecoder`, `Msg`, `AccAddress`, `Coins`, etc. | upstream (excluding `TxRunner`) |
| `github.com/cosmos/cosmos-sdk/client` | `TxConfig` | upstream |
| `github.com/cosmos/cosmos-sdk/crypto/types` | `PrivKey` | upstream |
| `cosmossdk.io/depinject` | `Config`, `Supply`, etc. | upstream |
| `cosmossdk.io/log` | `Logger`, `NewNopLogger` | upstream |
| `cosmossdk.io/core/store` | `KVStoreService` | upstream |
| `github.com/cosmos/cosmos-db` | `DB`, `NewMemDB` | upstream |
| `github.com/cometbft/cometbft/types` | `ValidatorSet` | upstream |

---

## Files with `//go:build sdk_hooks` to Remove

| File | What it contains | Destination after migration |
|------|-----------------|---------------------------|
| `run/command_sdk.go` | `init()` that sets `newExecutorFn` | moves to `cmd/blockstm-sim/sdkimpl/` |
| `run/executor.go` | `FixtureExecutor`, `initApp`, `buildTx`, `initGenesis` | split: core → `run/`, adapter → `cmd/.../sdkimpl/` |
| `run/repeat_executor.go` | `RepeatRunExecutor` | same split |
| `run/perturb.go` | `newSTMRunnerForProbe` | `cmd/.../sdkimpl/` (uses `txnrunner` directly) |
| `instrument/instrument_stm.go` | `InstrumentSTM`, `STMInstrumentable` | merged into `instrument/instrument.go` using `sdkhook.App` |
| `run/executor_canary.go` | canary-specific `init()` hooks | stays, loses `extraPopulateOracleTrackers` after reflect tracker lands |

---

## Proposed `sdkhook.App` Interface Surface

```go
type App interface {
    // From cometbft ABCI — upstream, already on runtime.App
    FinalizeBlock(*abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error)

    // BlockSTM control — added to runtime.App by SDK fork
    SetBlockSTMTxRunner(STMRunner)
    UnsetBlockSTMTxRunner()
    SetDisableBlockGasMeter(bool)

    // Observer — added to runtime.App by SDK fork
    SetLifecycleObserver(compare.LifecycleObserver)

    // Store access — used to wire STMRunner; may be upstream
    GetStoreKeys() []storetypes.StoreKey
}
```

---

*Input to `sdkhook/sdkhook.go` interface design.*
