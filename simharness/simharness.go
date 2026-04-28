package simharness

var (
	enabled       bool
	canaryEnabled bool
)

func Enabled() bool {
	return enabled
}

func CanaryEnabled() bool {
	return canaryEnabled
}
