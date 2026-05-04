package blockstm

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchedulerHook_FiresAtPhases(t *testing.T) {
	var calls []string
	var mu sync.Mutex
	hook := func(phase string) {
		mu.Lock()
		calls = append(calls, phase)
		mu.Unlock()
	}

	s := NewScheduler(1, WithHook(hook))
	require.NotNil(t, s)

	// Drive the scheduler through all 4 phase boundaries manually.
	version, kind := s.NextTask()
	require.True(t, version.Valid())
	require.Equal(t, TaskKindExecution, kind)

	// FinishExecution
	next, _ := s.FinishExecution(version, false)

	// TryValidationAbort — version is in EXECUTED state from FinishExecution
	_ = s.TryValidationAbort(version)

	// FinishValidation
	_, _ = s.FinishValidation(version.Index, true)

	mu.Lock()
	defer mu.Unlock()

	require.Contains(t, calls, "NextTask")
	require.Contains(t, calls, "FinishExecution")
	require.Contains(t, calls, "TryValidationAbort")
	require.Contains(t, calls, "FinishValidation")
	_ = next
}

func TestSchedulerHook_NilHookIsNoOp(t *testing.T) {
	// Scheduler without hook must not panic
	s := NewScheduler(1)
	version, _ := s.NextTask()
	if version.Valid() {
		s.FinishExecution(version, false)
	}
}
