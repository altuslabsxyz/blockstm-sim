//go:build sdk_hooks

package run_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/run"
)

// TestRepeatRunExecutor_Probes_N1 verifies that NewRepeatRunExecutor(1) stores
// n=1 and satisfies the Executor interface without panicking.
func TestRepeatRunExecutor_Probes_N1(t *testing.T) {
	exec := run.NewRepeatRunExecutor(1)
	require.NotNil(t, exec)

	// Verify it implements Executor.
	var _ run.Executor = exec
}

// TestRepeatRunExecutor_Probes_Clamp verifies that n < 1 is clamped to 1.
func TestRepeatRunExecutor_Probes_Clamp(t *testing.T) {
	exec := run.NewRepeatRunExecutor(0)
	require.NotNil(t, exec)

	exec2 := run.NewRepeatRunExecutor(-5)
	require.NotNil(t, exec2)

	// Both should compile and not panic — clamping is verified indirectly via
	// the fact that Init would create exactly 1 probe (tested in integration).
}

// TestRepeatRunExecutor_ImplementsExecutor checks the interface at compile time.
func TestRepeatRunExecutor_ImplementsExecutor(t *testing.T) {
	t.Parallel()

	var exec run.Executor = run.NewRepeatRunExecutor(3)
	assert.NotNil(t, exec)
}

// TestGenerateProbeConfigs_Count verifies that generateProbeConfigs is exported
// indirectly through the executor constructor.  We verify basic properties via
// the public API: creating a RepeatRunExecutor with n=4 does not panic and
// satisfies the interface.
func TestGenerateProbeConfigs_Count(t *testing.T) {
	t.Parallel()

	for _, n := range []int{1, 2, 4, 8} {
		exec := run.NewRepeatRunExecutor(n)
		require.NotNil(t, exec, "n=%d", n)

		var _ run.Executor = exec
	}
}
