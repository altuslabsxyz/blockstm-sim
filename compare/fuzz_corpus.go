package compare

import (
	"context"
	"iter"
	"math/rand"
	"sort"

	"github.com/altuslabsxyz/blockstm-sim/coverage"
)

// FuzzConfig parameterises a fuzz corpus run.
type FuzzConfig struct {
	Seed       int64          `json:"seed"`
	Blocks     int            `json:"blocks"`
	TxPerBlock int            `json:"tx_per_block"`
	Weights    map[string]int `json:"weights"`
	Genesis    GenesisSpec    `json:"genesis"`
}

// FuzzCorpus implements CorpusStore by generating deterministic random blocks
// from the registered coverage message types. Same seed always produces the
// same block sequence, making failures reproducible.
type FuzzCorpus struct {
	cfg  FuzzConfig
	name string
}

// NewFuzzCorpus creates a FuzzCorpus from the given config.
// If cfg.Genesis has no accounts, three default accounts are used.
func NewFuzzCorpus(cfg FuzzConfig) *FuzzCorpus {
	if len(cfg.Genesis.Accounts) == 0 {
		cfg.Genesis.Accounts = map[string]AccountSpec{
			"alice":   {Balance: "1000000000stake"},
			"bob":     {Balance: "1000000000stake"},
			"charlie": {Balance: "1000000000stake"},
		}
	}
	return &FuzzCorpus{cfg: cfg, name: "fuzz"}
}

// Iter generates blocks lazily. Each call re-seeds the RNG from cfg.Seed so
// repeated iterations are byte-identical.
func (fc *FuzzCorpus) Iter(ctx context.Context) iter.Seq2[Block, error] {
	return func(yield func(Block, error) bool) {
		rng := rand.New(rand.NewSource(fc.cfg.Seed)) //nolint:gosec
		keys, weights := fc.buildWeightedKeys()
		accounts := fc.sortedAccounts()
		if len(keys) == 0 || len(accounts) == 0 {
			return
		}

		for i := 0; i < fc.cfg.Blocks; i++ {
			if ctx.Err() != nil {
				yield(Block{}, ctx.Err())
				return
			}
			txs := make([]TxSpec, fc.cfg.TxPerBlock)
			for j := 0; j < fc.cfg.TxPerBlock; j++ {
				txs[j] = TxSpec{
					Signer: accounts[rng.Intn(len(accounts))],
					Msg:    weightedSample(rng, keys, weights),
					To:     accounts[rng.Intn(len(accounts))],
					Amount: "1" + fc.BondDenom(),
					Gas:    200000,
				}
			}
			if !yield(Block{Txs: txs}, nil) {
				return
			}
		}
	}
}

// buildWeightedKeys returns a sorted key slice and a parallel weight slice.
// If cfg.Weights is non-empty, only keys present in both cfg.Weights and the
// coverage registry are included; unregistered weight keys are silently skipped.
// If cfg.Weights is empty, all registered keys get equal weight.
func (fc *FuzzCorpus) buildWeightedKeys() (keys []string, weights []int) {
	registered := coverage.Registered()

	if len(fc.cfg.Weights) > 0 {
		for k := range fc.cfg.Weights {
			if _, ok := registered[k]; ok {
				keys = append(keys, k)
			}
		}
	} else {
		for k := range registered {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	weights = make([]int, len(keys))
	for i, k := range keys {
		if w, ok := fc.cfg.Weights[k]; ok {
			weights[i] = w
		} else {
			weights[i] = 1
		}
	}
	return keys, weights
}

func (fc *FuzzCorpus) sortedAccounts() []string {
	names := make([]string, 0, len(fc.cfg.Genesis.Accounts))
	for name := range fc.cfg.Genesis.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// weightedSample draws a key from keys proportional to weights using rng.
func weightedSample(rng *rand.Rand, keys []string, weights []int) string {
	total := 0
	for _, w := range weights {
		total += w
	}
	if total == 0 {
		return keys[0]
	}
	r := rng.Intn(total)
	for i, w := range weights {
		r -= w
		if r < 0 {
			return keys[i]
		}
	}
	return keys[len(keys)-1]
}

func (fc *FuzzCorpus) SnapshotDir() string  { return "" }
func (fc *FuzzCorpus) Meta() RangeMeta      { return RangeMeta{} }
func (fc *FuzzCorpus) BondDenom() string    { return "stake" }
func (fc *FuzzCorpus) Name() string         { return fc.name }
func (fc *FuzzCorpus) IsCanary() bool       { return false }
func (fc *FuzzCorpus) Genesis() GenesisSpec { return fc.cfg.Genesis }
func (fc *FuzzCorpus) BlockCount() int      { return fc.cfg.Blocks }
func (fc *FuzzCorpus) Close() error         { return nil }
