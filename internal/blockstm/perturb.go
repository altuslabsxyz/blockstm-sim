package blockstm

import (
	"math/rand"
	"sync"
	"time"
)

// NewPerturbHook returns a scheduler hook that sleeps for a random duration
// in [0, 50µs) at each phase boundary. The seed makes the delay sequence
// reproducible: the same seed always produces the same pattern, ensuring
// that failures triggered by the perturbation can be reproduced.
//
// The returned hook is goroutine-safe and may be called concurrently from
// multiple executor goroutines.
func NewPerturbHook(seed int64) func(string) {
	r := rand.New(rand.NewSource(seed)) //nolint:gosec // intentionally non-crypto
	var mu sync.Mutex
	return func(_ string) {
		mu.Lock()
		d := time.Duration(r.Intn(50)) * time.Microsecond
		mu.Unlock()
		// Sleep outside the lock so goroutines sleep independently,
		// preserving the scheduling perturbation effect.
		time.Sleep(d)
	}
}
