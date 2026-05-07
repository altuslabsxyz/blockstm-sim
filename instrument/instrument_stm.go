//go:build sdk_hooks

package instrument

import sdk "github.com/cosmos/cosmos-sdk/types"

type STMInstrumentable interface {
	Instrumentable
	SetBlockSTMTxRunner(sdk.TxRunner)
	SetDisableBlockGasMeter(bool)
}

func InstrumentSTM(app STMInstrumentable, runner sdk.TxRunner) {
	app.SetDisableBlockGasMeter(true)
	app.SetBlockSTMTxRunner(runner)
}
