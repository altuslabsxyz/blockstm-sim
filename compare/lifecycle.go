package compare

import abci "github.com/cometbft/cometbft/abci/types"

// LifecycleObserver receives block and transaction lifecycle events during
// FinalizeBlock execution. It mirrors the SDK fork's lifecycle.LifecycleObserver
// but is owned by blockstm-sim, allowing external integrations to depend on
// this package instead of the SDK fork directly.
type LifecycleObserver interface {
	OnFinalizeBlockStart(height int64)
	OnFinalizeBlockEnd(appHash []byte)
	OnTxStart(txIndex int)
	OnTxEnd(txIndex int, result *abci.ExecTxResult)
	OnKVWrite(storeKey string, key []byte, txIndex int)
}

// NoopLifecycleObserver is a LifecycleObserver that does nothing.
type NoopLifecycleObserver struct{}

func (NoopLifecycleObserver) OnFinalizeBlockStart(int64)      {}
func (NoopLifecycleObserver) OnFinalizeBlockEnd([]byte)       {}
func (NoopLifecycleObserver) OnTxStart(int)                   {}
func (NoopLifecycleObserver) OnTxEnd(int, *abci.ExecTxResult) {}
func (NoopLifecycleObserver) OnKVWrite(string, []byte, int)   {}
