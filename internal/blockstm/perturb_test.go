package blockstm

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewPerturbHook_DeterministicBySeed(t *testing.T) {
	hook1 := NewPerturbHook(42)
	hook2 := NewPerturbHook(42)

	phases := []string{"NextTask", "FinishExecution", "TryValidationAbort", "FinishValidation"}
	for _, p := range phases {
		start := time.Now()
		hook1(p)
		elapsed := time.Since(start)
		require.GreaterOrEqual(t, elapsed, time.Duration(0))
		require.Less(t, elapsed, 100*time.Microsecond, "delay should be bounded for phase %s", p)
	}
	for _, p := range phases {
		start := time.Now()
		hook2(p)
		elapsed := time.Since(start)
		require.GreaterOrEqual(t, elapsed, time.Duration(0))
		require.Less(t, elapsed, 100*time.Microsecond, "delay should be bounded for phase %s", p)
	}
}

func TestNewPerturbHook_DifferentSeedsDifferentDelays(t *testing.T) {
	hook1 := NewPerturbHook(1)
	hook2 := NewPerturbHook(2)
	hook1("NextTask")
	hook2("NextTask")
}

func TestNewPerturbHook_GoroutineSafe(t *testing.T) {
	hook := NewPerturbHook(99)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hook("NextTask")
			hook("FinishExecution")
		}()
	}
	wg.Wait()
}
