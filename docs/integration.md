# Chain-Side Integration Guide

This guide explains how to wire any Cosmos SDK chain into `blockstm-sim` for
parallel-execution safety testing. The target reader is a **chain maintainer**
who is adding blockstm-sim to an existing chain binary. Module developers in the
chain repo do not need to be aware of blockstm-sim.

---

## Prerequisites: SDK Fork Capabilities

`blockstm-sim` requires a cosmos-sdk fork with the following additions to
`*runtime.App`. These methods do not exist in upstream cosmos-sdk.

| Method | Purpose |
|--------|---------|
| `SetLifecycleObserver(LifecycleObserver)` | Wire observer for block/tx lifecycle events |
| `UnsetBlockSTMTxRunner()` | Remove the parallel runner (revert to sequential) |
| `SetBlockSTMTxRunner(TxRunner)` | Install the BlockSTM parallel runner |
| `SetDisableBlockGasMeter(bool)` | Disable block-level gas meter for simulation |
| `GetStoreKeys() []StoreKey` | Enumerate all multistore keys |

The fork must also provide:

- `baseapp/txnrunner` — `NewSTMRunner` constructor
- `baseapp/txnrunner` — `WithPerturbHook` scheduler-perturbation option
- `baseapp/lifecycle` — `LifecycleObserver` interface

blockstm-sim's `sdkhook` package defines its own interface boundary. The SDK
fork does **not** import `sdkhook`; structural typing handles the match.

---

## Integration Point: `cmd/<runner>/main.go`

All chain-specific wiring is done in a **single `init()` function** inside the
chain adapter package. The adapter is the only place in the chain codebase that
imports fork-specific packages (`txnrunner`, `lifecycle`, `runtime`).

Create (or extend) a file, typically `cmd/<runner>/sdkimpl/sdkimpl.go`:

```go
//go:build sdk_hooks

package sdkimpl

import (
    storetypes "cosmossdk.io/store/types"
    "github.com/cosmos/cosmos-sdk/baseapp/lifecycle"
    "github.com/cosmos/cosmos-sdk/baseapp/txnrunner"
    "github.com/cosmos/cosmos-sdk/runtime"
    sdk "github.com/cosmos/cosmos-sdk/types"

    "github.com/altuslabsxyz/blockstm-sim/compare"
    "github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// appAdapter bridges *runtime.App to sdkhook.App.
// It lives here so that no other package in blockstm-sim imports fork types.
type appAdapter struct{ *runtime.App }

// SetLifecycleObserver maps compare.LifecycleObserver → lifecycle.LifecycleObserver.
// The type assertion succeeds because the method sets are structurally identical.
func (a *appAdapter) SetLifecycleObserver(obs compare.LifecycleObserver) {
    a.App.SetLifecycleObserver(obs.(lifecycle.LifecycleObserver))
}

// SetBlockSTMTxRunner maps sdkhook.STMRunner (any) → sdk.TxRunner.
// The factory always returns *txnrunner.STMRunner, which implements sdk.TxRunner.
func (a *appAdapter) SetBlockSTMTxRunner(runner sdkhook.STMRunner) {
    a.App.SetBlockSTMTxRunner(runner.(sdk.TxRunner))
}

func init() {
    // 1. Register the module/keeper discovery function.
    //    Called once per oracle app setup to enumerate all modules for
    //    the reflective out-of-KV tracker.
    sdkhook.RegisterKeeperDiscovery(func(raw any) []any {
        app := raw.(*runtime.App)
        mods := make([]any, 0, len(app.ModuleManager.Modules))
        for _, mod := range app.ModuleManager.Modules {
            mods = append(mods, mod)
        }
        return mods
    })

    // 2. Register the app wrapper.
    //    Adapts *runtime.App into sdkhook.App for the simulation harness.
    sdkhook.RegisterAppWrapper(func(rawApp any) sdkhook.App {
        return &appAdapter{rawApp.(*runtime.App)}
    })

    // 3. Register the STM runner factory.
    //    perturbSeed == 0 → oracle / baseline probe (no perturbation).
    //    perturbSeed != 0 → probe mode with scheduler race-widening.
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

Import this package with a blank identifier from the runner `main.go` so that
`init()` fires at startup:

```go
//go:build sdk_hooks

package main

import (
    _ "github.com/<org>/<chain>/cmd/<runner>/sdkimpl"
)
```

> **Build tag**: The `sdk_hooks` build tag gates all fork-dependent code. The
> public `go.mod` points to upstream cosmos-sdk; CI uses `go.internal.mod`
> with the fork's replace directives when running full simulation tests.

---

## Registrations Summary

Three functions must be called from `init()` before any executor is
constructed. Each panics if called more than once to catch double-registration.

| Function | What it provides |
|----------|-----------------|
| `sdkhook.RegisterAppWrapper(f)` | `*runtime.App` → `sdkhook.App` bridge |
| `sdkhook.RegisterSTMRunnerFactory(f)` | parallel runner construction |
| `sdkhook.RegisterKeeperDiscovery(f)` | module enumeration for reflect tracker |

---

## Reflective Out-of-KV Tracker

After oracle app setup, `blockstm-sim` calls `sdkhook.DiscoverKeepers(rawApp)`,
receives the list of module instances from the registered discovery function,
and wraps each one in a `tracker.KeeperReflectTracker`.

The tracker snapshots mutable, non-KV state via reflection:

- **Included**: plain `map[K]V`, `[]T`, `int`/`uint`/`bool`/`string` scalars,
  nested structs, `sync.Map`, `atomic.*`
- **Excluded automatically**:
  - Interface fields (concrete type unknown at snapshot time)
  - `func`, `chan`, `unsafe.Pointer` fields
  - `sync.Mutex` / `sync.RWMutex` struct values
  - Types from the following packages (KV-backed or immutable config):

| Excluded package prefix | Reason |
|------------------------|--------|
| `cosmossdk.io/core/store` | KVStore-backed |
| `cosmossdk.io/collections` | KVStore-backed |
| `cosmossdk.io/schema` | Schema metadata (immutable) |
| `cosmossdk.io/log` | Logger (not mutable state) |
| `cosmossdk.io/depinject` | DI internals |
| `github.com/cosmos/cosmos-sdk/codec` | Codec (immutable) |
| `github.com/cosmos/cosmos-sdk/baseapp` | App internals |
| `github.com/cosmos/cosmos-sdk/types/module` | Module manager |
| `github.com/cosmos/cosmos-sdk/client` | Client config |
| `github.com/cosmos/cosmos-sdk/server` | Server config |
| `github.com/cometbft/cometbft` | Consensus internals |
| `github.com/gogo/protobuf` | Protobuf runtime |
| `google.golang.org/grpc` | gRPC internals |
| `google.golang.org/protobuf` | Protobuf runtime |

Unexported fields are accessible via `unsafe.Pointer` + `reflect.NewAt`, so
the tracker detects mutations to private fields without any change to the
keeper source.

---

## Reducing False Positives

On first integration, the tracker will emit findings for every module whose
fields change between oracle transactions. Many of these are expected and
benign (e.g. a cache field updated deterministically by both oracle and probe).

Strategies to reduce noise:

1. **Inspect the tracker name in findings.** Each F4 finding's `Tracker` field
   contains the fully-qualified Go type name (e.g.
   `github.com/<org>/<chain>/x/<module>.AppModule`). Use this to identify
   which modules are emitting findings.

2. **Look at the snapshot diff.** The `Oracle` and `Probe` fields in a finding
   contain `<trackerName>:<hex(snapshot)>`. Decode the hex to see which fields
   are changing. Field names are embedded as `[FieldName]` labels in the
   snapshot bytes.

3. **Extend the deny-list for chain-specific packages.** If a module's fields
   are legitimately not out-of-KV state (e.g. a deterministic cache that is
   always consistent), add the module's package prefix to the deny list in
   `tracker/filter.go`:

   ```go
   var denyPkgPrefixes = []string{
       // ... existing entries ...
       "github.com/<org>/<chain>/x/<module>", // deterministic cache, not out-of-KV state
   }
   ```

   > **Note**: This modifies blockstm-sim itself. A chain-side registration API
   > for extending the deny list is planned for a future release.

---

## Test Registration (Unit Tests)

Integration tests that do not import the cmd package must register the hooks
themselves, typically in `TestMain`:

```go
func TestMain(m *testing.M) {
    sdkhook.RegisterKeeperDiscovery(func(raw any) []any {
        app := raw.(*runtime.App)
        mods := make([]any, 0, len(app.ModuleManager.Modules))
        for _, mod := range app.ModuleManager.Modules {
            mods = append(mods, mod)
        }
        return mods
    })

    sdkhook.RegisterAppWrapper(func(raw any) sdkhook.App {
        return &testAppAdapter{raw.(*runtime.App)}
    })

    sdkhook.RegisterSTMRunnerFactory(func(
        decoder sdk.TxDecoder,
        keys []storetypes.StoreKey,
        workers int,
        perturbSeed int64,
    ) sdkhook.STMRunner {
        // test-specific runner construction
    })

    os.Exit(m.Run())
}
```

See `run/setup_test.go` for the reference implementation used by blockstm-sim's
own test suite.
