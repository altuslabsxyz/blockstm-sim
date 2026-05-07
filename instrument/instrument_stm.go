package instrument

import "github.com/altuslabsxyz/blockstm-sim/sdkhook"

// InstrumentSTM installs runner into app for parallel BlockSTM execution.
// Both app and runner are provided by the chain adapter via sdkhook registration.
func InstrumentSTM(app sdkhook.App, runner sdkhook.STMRunner) {
	app.SetDisableBlockGasMeter(true)
	app.SetBlockSTMTxRunner(runner)
}
