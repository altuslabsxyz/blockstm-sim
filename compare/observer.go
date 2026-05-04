package compare

import (
	"encoding/hex"
	"sort"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/cosmos-sdk/baseapp/lifecycle"
)

// WriteSetProvider returns the sorted write keys for a given transaction.
type WriteSetProvider interface {
	TxWriteSet(txIndex int) []string
}

// BlockObserver implements lifecycle.LifecycleObserver to capture
// per-transaction KVStore write keys during FinalizeBlock execution.
//
// Each transaction index owns its own map, so concurrent OnKVWrite calls
// from different goroutines (BlockSTM incarnations) are safe without a mutex
// as long as each goroutine writes to a distinct txIndex.
type BlockObserver struct {
	lifecycle.NoopLifecycleObserver
	writeSets []map[string]struct{}
}

var _ lifecycle.LifecycleObserver = (*BlockObserver)(nil)
var _ WriteSetProvider = (*BlockObserver)(nil)

func NewBlockObserver(txCount int) *BlockObserver {
	ws := make([]map[string]struct{}, txCount)
	for i := range ws {
		ws[i] = make(map[string]struct{})
	}
	return &BlockObserver{writeSets: ws}
}

func (o *BlockObserver) OnTxStart(txIndex int) {
	if txIndex < len(o.writeSets) {
		o.writeSets[txIndex] = make(map[string]struct{})
	}
}

func (o *BlockObserver) OnKVWrite(storeKey string, key []byte, txIndex int) {
	if txIndex < len(o.writeSets) {
		composite := storeKey + "/" + hex.EncodeToString(key)
		o.writeSets[txIndex][composite] = struct{}{}
	}
}

func (o *BlockObserver) OnTxEnd(_ int, _ *abci.ExecTxResult) {}

// TxWriteSet returns the sorted list of composite write keys for the given tx.
func (o *BlockObserver) TxWriteSet(txIndex int) []string {
	if txIndex >= len(o.writeSets) {
		return nil
	}
	m := o.writeSets[txIndex]
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
