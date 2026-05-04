package compare

import (
	"context"
	"iter"

	dbm "github.com/cosmos/cosmos-db"
)

// Block is a type alias for BlockSpec, representing a single block in a corpus.
type Block = BlockSpec

// CorpusStore provides sequential access to blocks and their pre-state.
type CorpusStore interface {
	Iter(ctx context.Context) iter.Seq2[Block, error]
	PreStateDB() dbm.DB
	BondDenom() string
	Name() string
	IsCanary() bool
	Genesis() GenesisSpec
	BlockCount() int
	Close() error
}

// LoadCorpusStores loads all YAML fixture files from dir and wraps each as a CorpusStore.
func LoadCorpusStores(dir string) ([]CorpusStore, error) {
	fixtures, err := LoadCorpus(dir)
	if err != nil {
		return nil, err
	}
	stores := make([]CorpusStore, len(fixtures))
	for i, f := range fixtures {
		stores[i] = NewFixtureCorpus(f)
	}
	return stores, nil
}
