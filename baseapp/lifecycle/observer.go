package lifecycle

import abci "github.com/cometbft/cometbft/abci/types"

var _ LifecycleObserver = NoopLifecycleObserver{}

// LifecycleObserver receives block and transaction lifecycle events.
type LifecycleObserver interface {
	OnFinalizeBlockStart(height int64)
	OnFinalizeBlockEnd(appHash []byte)
	OnTxStart(txIndex int)
	OnTxEnd(txIndex int, result *abci.ExecTxResult)
	OnKVWrite(storeKey string, key []byte, txIndex int)
}

// NoopLifecycleObserver is a LifecycleObserver that does nothing.
// It is used as the default when no observer is installed.
type NoopLifecycleObserver struct{}

func (NoopLifecycleObserver) OnFinalizeBlockStart(int64)      {}
func (NoopLifecycleObserver) OnFinalizeBlockEnd([]byte)       {}
func (NoopLifecycleObserver) OnTxStart(int)                   {}
func (NoopLifecycleObserver) OnTxEnd(int, *abci.ExecTxResult) {}
func (NoopLifecycleObserver) OnKVWrite(string, []byte, int)   {}
