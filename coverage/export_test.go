package coverage

// ClearRegistry removes all registered entries. For use in tests only.
var ClearRegistry = func() {
	globalMu.Lock()
	defer globalMu.Unlock()
	global = map[string]Entry{}
}
