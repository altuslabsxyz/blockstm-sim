package keeper

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"cosmossdk.io/core/store"
)

// SetValueDelay, if positive, is inserted in SetMapValue before the mutex
// acquisition. This widens the BlockSTM race window for C1 canary testing by
// giving a concurrent MapReadAndWrite goroutine time to read the stale
// zero-value before MapSet's write lands.
var SetValueDelay time.Duration

// BlockContextWriter is implemented by the compare block-context tracker
// without requiring this package to import compare.
type BlockContextWriter interface {
	ReadField(name string) string
	WriteField(name, value string)
}

type Keeper struct {
	storeService store.KVStoreService

	// sharedMap is intentionally outside MVMemory tracking.
	// Under BlockSTM parallel execution, concurrent reads and writes
	// to this map produce a stale-read violation that diverges AppHash.
	sharedMap map[string]int64
	mu        sync.Mutex

	blockCtxWriter BlockContextWriter
}

func NewKeeper(ss store.KVStoreService) *Keeper {
	return &Keeper{
		storeService: ss,
		sharedMap:    make(map[string]int64),
	}
}

func (k *Keeper) SetMapValue(key string, value int64) {
	if SetValueDelay > 0 {
		time.Sleep(SetValueDelay)
	}
	k.mu.Lock()
	k.sharedMap[key] = value
	k.mu.Unlock()
}

func (k *Keeper) GetMapValue(key string) int64 {
	k.mu.Lock()
	v := k.sharedMap[key]
	k.mu.Unlock()
	return v
}

func (k *Keeper) SetBlockContextWriter(w BlockContextWriter) {
	k.blockCtxWriter = w
}

func (k *Keeper) WriteBlockCtxField(name, value string) {
	if k.blockCtxWriter == nil {
		return
	}
	k.blockCtxWriter.WriteField(name, value)
}

func (k *Keeper) ReadBlockCtxField(name string) string {
	if k.blockCtxWriter == nil {
		return ""
	}
	return k.blockCtxWriter.ReadField(name)
}

func (k *Keeper) WriteToStore(ctx context.Context, key string, value int64) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(value))
	return kvStore.Set([]byte("canary/"+key), buf)
}

