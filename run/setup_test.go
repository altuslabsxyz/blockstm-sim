//go:build sdk_hooks

package run

import (
	"os"
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/baseapp/lifecycle"
	"github.com/cosmos/cosmos-sdk/baseapp/txnrunner"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// TestMain registers sdkhook factories before any test in the run package runs.
// In production, cmd/blockstm-sim/sdkimpl registers these via init(); tests do
// not import that cmd package, so registration happens here instead.
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
		if perturbSeed == 0 {
			return txnrunner.NewSTMRunner(decoder, keys, workers, false, nil)
		}
		return txnrunner.NewSTMRunner(decoder, keys, workers, false, nil,
			txnrunner.WithPerturbHook(perturbSeed))
	})
	os.Exit(m.Run())
}

// testAppAdapter mirrors sdkimpl.appAdapter for tests.
type testAppAdapter struct{ *runtime.App }

func (a *testAppAdapter) SetLifecycleObserver(obs compare.LifecycleObserver) {
	a.App.SetLifecycleObserver(obs.(lifecycle.LifecycleObserver))
}

func (a *testAppAdapter) SetBlockSTMTxRunner(runner sdkhook.STMRunner) {
	a.App.SetBlockSTMTxRunner(runner.(sdk.TxRunner))
}

func (a *testAppAdapter) CommitMultiStore() storetypes.CommitMultiStore {
	return a.App.CommitMultiStore()
}
