package run

// probeConfig describes perturbation parameters for one repeat-run probe.
type probeConfig struct {
	workers int
	seed    int64 // 0 = no perturbation (baseline)
}

// generateProbeConfigs returns n deterministic probe configurations for F2 repeat-run checking.
// Config 0: no perturbation, 4 workers (baseline probe).
// Config i (i>0): seed=int64(i), workers = 4 + (i-1)%4 (cycles 4..7).
func generateProbeConfigs(n int) []probeConfig {
	if n < 1 {
		n = 1
	}
	configs := make([]probeConfig, n)
	for i := range n {
		if i == 0 {
			configs[0] = probeConfig{workers: 4, seed: 0}
		} else {
			configs[i] = probeConfig{
				workers: 4 + (i-1)%4,
				seed:    int64(i),
			}
		}
	}
	return configs
}
