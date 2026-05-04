//go:build sdk_hooks

package run

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateProbeConfigs_Deterministic(t *testing.T) {
	// calling twice with same n returns identical configs
	a := generateProbeConfigs(5)
	b := generateProbeConfigs(5)
	require.Equal(t, a, b)
}

func TestGenerateProbeConfigs_WorkerVariety(t *testing.T) {
	configs := generateProbeConfigs(5)
	for _, c := range configs {
		require.GreaterOrEqual(t, c.workers, 4)
		require.LessOrEqual(t, c.workers, 7)
	}
}

func TestGenerateProbeConfigs_SeedVariety(t *testing.T) {
	configs := generateProbeConfigs(5)
	// config 0 is the unperturbed baseline (seed=0)
	require.Equal(t, int64(0), configs[0].seed)
	// configs 1..N-1 have distinct non-zero seeds
	seen := map[int64]bool{}
	for _, c := range configs[1:] {
		require.NotZero(t, c.seed)
		require.False(t, seen[c.seed], "seed %d appeared twice", c.seed)
		seen[c.seed] = true
	}
}

func TestGenerateProbeConfigs_MinN(t *testing.T) {
	// n<1 clamps to 1
	configs := generateProbeConfigs(0)
	require.Len(t, configs, 1)
	require.Equal(t, int64(0), configs[0].seed)
}
