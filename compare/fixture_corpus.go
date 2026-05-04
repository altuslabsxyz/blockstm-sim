package compare

import (
	"context"
	"iter"

	dbm "github.com/cosmos/cosmos-db"
)

// FixtureCorpus wraps a Fixture to implement the CorpusStore interface.
type FixtureCorpus struct {
	fixture Fixture
}

// NewFixtureCorpus creates a FixtureCorpus from the given Fixture.
func NewFixtureCorpus(f Fixture) *FixtureCorpus {
	return &FixtureCorpus{fixture: f}
}

// Iter returns an iterator over the fixture's blocks.
func (fc *FixtureCorpus) Iter(ctx context.Context) iter.Seq2[Block, error] {
	return func(yield func(Block, error) bool) {
		for _, block := range fc.fixture.Blocks {
			if ctx.Err() != nil {
				yield(Block{}, ctx.Err())
				return
			}
			if !yield(block, nil) {
				return
			}
		}
	}
}

// PreStateDB returns nil for fixture-based corpora (state is built from genesis).
func (fc *FixtureCorpus) PreStateDB() dbm.DB { return nil }

// BondDenom returns the default bond denomination.
func (fc *FixtureCorpus) BondDenom() string { return "stake" }

// Name returns the fixture name.
func (fc *FixtureCorpus) Name() string { return fc.fixture.Name }

// IsCanary reports whether the fixture is a canary fixture.
func (fc *FixtureCorpus) IsCanary() bool { return fc.fixture.IsCanary() }

// Genesis returns the fixture's genesis specification.
func (fc *FixtureCorpus) Genesis() GenesisSpec { return fc.fixture.Genesis }

// BlockCount returns the number of blocks in the fixture.
func (fc *FixtureCorpus) BlockCount() int { return len(fc.fixture.Blocks) }

// Close is a no-op for fixture-based corpora.
func (fc *FixtureCorpus) Close() error { return nil }
