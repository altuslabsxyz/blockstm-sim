// Package sdkhook defines the interface boundary between blockstm-sim and the
// cosmos-sdk fork that provides BlockSTM parallel execution.
//
// The SDK fork implements these interfaces structurally — it does not need to
// import this package. blockstm-sim core uses only these types; no direct
// imports of fork-specific packages (txnrunner, sdk.TxRunner, runtime.App)
// appear outside of chain-specific adapter code.
package sdkhook

import (
	abci "github.com/cometbft/cometbft/abci/types"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// App is the minimal interface blockstm-sim requires from a cosmos-sdk runtime.App.
// The SDK fork's *runtime.App satisfies this structurally once the BlockSTM hook
// methods are present. No import of this package is required on the SDK side.
type App interface {
	// FinalizeBlock executes a block of transactions and returns the results.
	FinalizeBlock(*abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error)

	// SetLifecycleObserver wires an observer to receive block/tx lifecycle events.
	// Pass compare.NoopLifecycleObserver{} to detach.
	SetLifecycleObserver(compare.LifecycleObserver)

	// SetBlockSTMTxRunner installs the BlockSTM parallel runner.
	SetBlockSTMTxRunner(STMRunner)

	// UnsetBlockSTMTxRunner removes the BlockSTM runner, reverting to sequential
	// execution. Used to put the oracle app into deterministic baseline mode.
	UnsetBlockSTMTxRunner()

	// SetDisableBlockGasMeter disables the block-level gas meter.
	// Required for simulation runs where gas accounting would interfere.
	SetDisableBlockGasMeter(bool)

	// GetStoreKeys returns all store keys registered with the app.
	// Used to wire the STMRunner to the multistore.
	GetStoreKeys() []storetypes.StoreKey
}

// STMRunner is the parallel BlockSTM transaction runner.
// blockstm-sim treats it as opaque: constructed via STMRunnerFactory and passed
// to App.SetBlockSTMTxRunner. sdk.TxRunner (fork-only) is not referenced here so
// that sdkhook compiles against upstream cosmos-sdk. The sdkimpl adapter performs
// the concrete type assertion when wiring into runtime.App.
type STMRunner interface{}

// STMRunnerFactory constructs an STMRunner for the given configuration.
//
// perturbSeed == 0: no scheduler perturbation (oracle or baseline probe).
// perturbSeed != 0: enables race-widening perturbation for probe mode.
//
// Registered by the chain adapter's init() in its cmd entry point.
type STMRunnerFactory func(
	decoder sdk.TxDecoder,
	keys []storetypes.StoreKey,
	workers int,
	perturbSeed int64,
) STMRunner

// AppWrapFunc adapts a raw SDK app (e.g. *runtime.App) into an sdkhook.App.
// Required because the SDK fork's SetLifecycleObserver may use an internal
// lifecycle type until the fork adopts sdkhook's LifecycleObserver directly.
// Registered by the chain adapter's init() in its cmd entry point.
type AppWrapFunc func(rawApp any) App

var appWrapFn AppWrapFunc

// RegisterAppWrapper registers the AppWrapFunc provided by the chain adapter.
// Panics if called more than once.
func RegisterAppWrapper(f AppWrapFunc) {
	if appWrapFn != nil {
		panic("sdkhook: AppWrapper already registered")
	}
	appWrapFn = f
}

// WrapApp wraps a raw SDK app using the registered AppWrapFunc.
// Panics if no wrapper has been registered.
func WrapApp(rawApp any) App {
	if appWrapFn == nil {
		panic("sdkhook: no AppWrapper registered; call RegisterAppWrapper in your adapter init()")
	}
	return appWrapFn(rawApp)
}

// runnerFactory is the registered STMRunnerFactory. Nil until RegisterSTMRunnerFactory
// is called by the chain adapter's init().
var runnerFactory STMRunnerFactory

// RegisterSTMRunnerFactory registers the STMRunnerFactory provided by the chain
// adapter. Must be called before any Executor is constructed. Panics if called
// more than once to catch accidental double-registration.
func RegisterSTMRunnerFactory(f STMRunnerFactory) {
	if runnerFactory != nil {
		panic("sdkhook: STMRunnerFactory already registered")
	}
	runnerFactory = f
}

// NewSTMRunner constructs an STMRunner using the registered factory.
// Panics if no factory has been registered.
func NewSTMRunner(decoder sdk.TxDecoder, keys []storetypes.StoreKey, workers int, perturbSeed int64) STMRunner {
	if runnerFactory == nil {
		panic("sdkhook: no STMRunnerFactory registered; call RegisterSTMRunnerFactory in your adapter init()")
	}
	return runnerFactory(decoder, keys, workers, perturbSeed)
}
