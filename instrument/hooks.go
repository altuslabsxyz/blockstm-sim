package instrument

import "github.com/altuslabsxyz/blockstm-sim/compare"

// AppHook is implemented by any app that supports lifecycle observation.
// External SDK integrations implement this interface to connect blockstm-sim's
// observer callbacks to their app's lifecycle system — without a direct
// dependency on the Altus cosmos-sdk fork's internal lifecycle types.
//
// The cosmos-sdk fork's runtime.App satisfies this interface indirectly via
// the sdk_hooks adapter in instrument.go.
type AppHook interface {
	SetObserver(obs compare.LifecycleObserver)
	UnsetRunner()
}
