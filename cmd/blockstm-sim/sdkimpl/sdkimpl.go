//go:build sdk_hooks

// Package sdkimpl registers the cosmos-sdk fork's concrete implementations of
// the sdkhook interfaces. This is the only package in blockstm-sim that imports
// fork-specific packages (txnrunner, lifecycle). All other packages use sdkhook interfaces.
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

// Compile-time guard: if the SDK fork adds a method to lifecycle.LifecycleObserver
// that is not present in compare.LifecycleObserver, this line fails to compile,
// catching the drift before it causes a runtime panic in SetLifecycleObserver.
var _ lifecycle.LifecycleObserver = compare.NoopLifecycleObserver{}

// appAdapter wraps *runtime.App to implement sdkhook.App.
// It bridges compare.LifecycleObserver (blockstm-sim's type) and
// lifecycle.LifecycleObserver (the SDK fork's type). Both interfaces have
// identical method sets, so any concrete observer satisfies both.
type appAdapter struct{ *runtime.App }

// SetLifecycleObserver adapts compare.LifecycleObserver → lifecycle.LifecycleObserver.
// The type assertion succeeds because the method sets are identical and any concrete
// value implementing one also implements the other.
func (a *appAdapter) SetLifecycleObserver(obs compare.LifecycleObserver) {
	a.App.SetLifecycleObserver(obs.(lifecycle.LifecycleObserver))
}

// SetBlockSTMTxRunner adapts sdkhook.STMRunner (interface{}) → sdk.TxRunner.
// The runner was created by the registered STMRunnerFactory which returns
// *txnrunner.STMRunner — a concrete type that implements sdk.TxRunner.
func (a *appAdapter) SetBlockSTMTxRunner(runner sdkhook.STMRunner) {
	a.App.SetBlockSTMTxRunner(runner.(sdk.TxRunner))
}

func (a *appAdapter) CommitMultiStore() storetypes.CommitMultiStore {
	return a.App.CommitMultiStore()
}

func init() {
	sdkhook.RegisterKeeperDiscovery(func(raw any) []any {
		app := raw.(*runtime.App)
		mods := make([]any, 0, len(app.ModuleManager.Modules))
		for _, mod := range app.ModuleManager.Modules {
			mods = append(mods, mod)
		}
		return mods
	})

	sdkhook.RegisterAppWrapper(func(rawApp any) sdkhook.App {
		return &appAdapter{rawApp.(*runtime.App)}
	})

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
