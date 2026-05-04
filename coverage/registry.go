package coverage

var global = map[string]Entry{}

// Register adds an Entry to the global registry.
// Call from init() in packages that own the message types.
func Register(key string, e Entry) {
	global[key] = e
}

// Registered returns a snapshot copy of the global registry.
func Registered() map[string]Entry {
	out := make(map[string]Entry, len(global))
	for k, v := range global {
		out[k] = v
	}
	return out
}
