package coverage

import "sync"

var (
	globalMu sync.RWMutex
	global   = map[string]Entry{}
)

// Register adds an Entry to the global registry.
// Call from init() in packages that own the message types.
func Register(key string, e Entry) {
	globalMu.Lock()
	defer globalMu.Unlock()
	global[key] = e
}

// ClearRegistry removes all registered entries.
// Intended for use in tests only.
func ClearRegistry() {
	globalMu.Lock()
	defer globalMu.Unlock()
	global = map[string]Entry{}
}

// Registered returns a snapshot copy of the global registry.
func Registered() map[string]Entry {
	globalMu.RLock()
	defer globalMu.RUnlock()
	out := make(map[string]Entry, len(global))
	for k, v := range global {
		out[k] = v
	}
	return out
}
