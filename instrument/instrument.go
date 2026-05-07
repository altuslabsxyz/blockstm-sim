package instrument

import "github.com/altuslabsxyz/blockstm-sim/compare"

type RunnerMode int

const (
	RunnerSTM RunnerMode = iota
	RunnerSequential
)

type Options struct {
	Observer compare.LifecycleObserver
	Runner   RunnerMode
}

// Instrumentable is the minimal interface required to instrument an app's runner
// mode. Observer injection uses a dynamic interface check so that types not
// implementing SetLifecycleObserver(compare.LifecycleObserver) are still
// accepted; those callers must set the observer directly.
type Instrumentable interface {
	UnsetBlockSTMTxRunner()
}

func InstrumentApp(app Instrumentable, opts Options) {
	if opts.Observer != nil {
		type observerSetter interface {
			SetLifecycleObserver(compare.LifecycleObserver)
		}
		if setter, ok := app.(observerSetter); ok {
			setter.SetLifecycleObserver(opts.Observer)
		}
	}
	if opts.Runner == RunnerSequential {
		app.UnsetBlockSTMTxRunner()
	}
}
