package instrument

import (
	"github.com/cosmos/cosmos-sdk/baseapp/lifecycle"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

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

type STMInstrumentable interface {
	Instrumentable
	SetBlockSTMTxRunner(sdk.TxRunner)
	SetDisableBlockGasMeter(bool)
}

func InstrumentApp(app Instrumentable, opts Options) {
	if opts.Observer != nil {
		app.SetLifecycleObserver(opts.Observer)
	}
	if opts.Runner == RunnerSequential {
		app.UnsetBlockSTMTxRunner()
	}
}

func InstrumentSTM(app STMInstrumentable, runner sdk.TxRunner) {
	app.SetDisableBlockGasMeter(true)
	app.SetBlockSTMTxRunner(runner)
}
