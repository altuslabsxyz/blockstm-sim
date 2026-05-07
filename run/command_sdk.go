//go:build sdk_hooks

package run

func init() {
	newExecutorFn = func(probes int) Executor {
		if probes > 1 {
			return NewRepeatRunExecutor(probes)
		}
		return NewFixtureExecutor()
	}
}
