package coverage

// ClearRegistry removes all registered entries. For use in tests only.
var ClearRegistry = func() {
	global = map[string]Entry{}
}
