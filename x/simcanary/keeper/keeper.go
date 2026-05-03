package keeper

import (
	"context"
	"encoding/binary"
	"sync"

	"cosmossdk.io/core/store"
)

type Keeper struct {
	storeService store.KVStoreService

	// sharedMap is intentionally outside MVMemory tracking.
	// Under BlockSTM parallel execution, concurrent reads and writes
	// to this map produce a stale-read violation that diverges AppHash.
	sharedMap map[string]int64
	mu        sync.Mutex
}

func NewKeeper(ss store.KVStoreService) *Keeper {
	return &Keeper{
		storeService: ss,
		sharedMap:    make(map[string]int64),
	}
}

func (k *Keeper) SetMapValue(key string, value int64) {
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

func (k *Keeper) WriteToStore(ctx context.Context, key string, value int64) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(value))
	return kvStore.Set([]byte("canary/"+key), buf)
}
