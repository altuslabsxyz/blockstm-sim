package compare

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"os"
	"path/filepath"

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

// LoadCorpusStores loads corpora from dir.
// If dir contains a fuzz.json file, it returns a single FuzzCorpus.
// Otherwise, it loads all *.yaml fixture files and wraps each as a FixtureCorpus.
func LoadCorpusStores(dir string) ([]CorpusStore, error) {
	fuzzPath := filepath.Join(dir, "fuzz.json")
	if _, err := os.Stat(fuzzPath); err == nil {
		return loadFuzzCorpusStore(fuzzPath)
	}

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

func loadFuzzCorpusStore(path string) ([]CorpusStore, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fuzz.json: %w", err)
	}
	var cfg FuzzConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse fuzz.json: %w", err)
	}
	return []CorpusStore{NewFuzzCorpus(cfg)}, nil
}
