package keeper

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"sort"
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

// OnKeeperCreated, if non-nil, is called whenever a new Keeper is constructed.
// The run package sets this to register the oracle's keeper for F4 mutation tracking,
// bypassing depinject output injection which may not be supported by all SDK builds.
var OnKeeperCreated func(*Keeper)

func NewKeeper(ss store.KVStoreService) *Keeper {
	k := &Keeper{
		storeService: ss,
		sharedMap:    make(map[string]int64),
	}
	if OnKeeperCreated != nil {
		OnKeeperCreated(k)
	}
	return k
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

func (k *Keeper) TrackerName() string { return "simcanary.sharedMap" }

func (k *Keeper) SnapshotOutOfKVStoreState() []byte {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.sharedMap) == 0 {
		return nil
	}
	keys := make([]string, 0, len(k.sharedMap))
	for key := range k.sharedMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	valBuf := make([]byte, 8)
	for _, key := range keys {
		binary.BigEndian.PutUint64(valBuf, uint64(k.sharedMap[key]))
		buf.WriteString(key)
		buf.WriteByte('=')
		buf.WriteString(hex.EncodeToString(valBuf))
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}
