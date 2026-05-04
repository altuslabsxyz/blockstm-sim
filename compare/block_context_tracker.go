package compare

import (
	"strconv"
	"strings"
	"sync"
)

// TxIndexSetter is implemented by components that need the currently executing
// oracle transaction index.
type TxIndexSetter interface {
	SetCurrentTx(txIndex int)
}

// BlockContextMutation records a block-context field mutation and the
// transactions that read the field before that write.
type BlockContextMutation struct {
	Field     string
	Before    string
	After     string
	WriterTx  int
	ReaderTxs []int
}

// BlockContextMutationProvider returns block-context mutations accumulated
// during block execution.
type BlockContextMutationProvider interface {
	BlockContextMutations() []BlockContextMutation
}

type blockContextField struct {
	value   string
	readers []int
}

// BlockContextTracker instruments block-context fields that live outside
// MVMemory, such as height and chain ID.
type BlockContextTracker struct {
	mu        sync.Mutex
	fields    map[string]blockContextField
	currentTx int
	mutations []BlockContextMutation
}

func NewBlockContextTracker(initial map[string]string) *BlockContextTracker {
	t := &BlockContextTracker{currentTx: -1}
	t.Reset(initial)
	return t
}

func (t *BlockContextTracker) Reset(initial map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.fields = make(map[string]blockContextField, len(initial))
	for k, v := range initial {
		t.fields[k] = blockContextField{value: v}
	}
	t.currentTx = -1
	t.mutations = nil
}

func (t *BlockContextTracker) SetCurrentTx(txIndex int) {
	t.mu.Lock()
	t.currentTx = txIndex
	t.mu.Unlock()
}

func (t *BlockContextTracker) ReadField(name string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	field, ok := t.fields[name]
	if !ok {
		return ""
	}
	if t.currentTx >= 0 {
		n := len(field.readers)
		if n == 0 || field.readers[n-1] != t.currentTx {
			field.readers = append(field.readers, t.currentTx)
			t.fields[name] = field
		}
	}
	return field.value
}

func (t *BlockContextTracker) WriteField(name, value string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	field := t.fields[name]
	readers := append([]int(nil), field.readers...)
	t.mutations = append(t.mutations, BlockContextMutation{
		Field:     name,
		Before:    field.value,
		After:     value,
		WriterTx:  t.currentTx,
		ReaderTxs: readers,
	})
	field.value = value
	t.fields[name] = field
}

func (t *BlockContextTracker) BlockContextMutations() []BlockContextMutation {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]BlockContextMutation, len(t.mutations))
	for i, mutation := range t.mutations {
		mutation.ReaderTxs = append([]int(nil), mutation.ReaderTxs...)
		out[i] = mutation
	}
	return out
}

func (t *BlockContextTracker) TrackerName() string { return "block.context" }

func (t *BlockContextTracker) SnapshotOutOfKVStoreState() []byte {
	return nil
}

func joinInts(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}
