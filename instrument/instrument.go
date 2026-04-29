package instrument

import "github.com/cosmos/cosmos-sdk/baseapp/lifecycle"

type RunnerMode int

const (
	RunnerSTM        RunnerMode = iota
	RunnerSequential
)

type Options struct {
	Observer lifecycle.LifecycleObserver
	Runner   RunnerMode
}

type Instrumentable interface {
	SetLifecycleObserver(lifecycle.LifecycleObserver)
	UnsetBlockSTMTxRunner()
}

func InstrumentApp(app Instrumentable, opts Options) {
	if opts.Observer != nil {
		app.SetLifecycleObserver(opts.Observer)
	}
	if opts.Runner == RunnerSequential {
		app.UnsetBlockSTMTxRunner()
	}
}
