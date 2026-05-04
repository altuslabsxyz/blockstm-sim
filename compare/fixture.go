package compare

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"sigs.k8s.io/yaml"
)

type FixtureKind string

const KindCanary FixtureKind = "canary"

type Fixture struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Kind        FixtureKind `json:"kind,omitempty"`
	Genesis     GenesisSpec `json:"genesis"`
	Blocks      []BlockSpec `json:"blocks"`
}

func (f Fixture) IsCanary() bool { return f.Kind == KindCanary }

type GenesisSpec struct {
	Accounts map[string]AccountSpec `json:"accounts"`
}

type AccountSpec struct {
	Balance string `json:"balance"`
}

type BlockSpec struct {
	Txs    []TxSpec `json:"txs,omitempty"`
	RawTxs [][]byte `json:"raw_txs,omitempty"`
	Height int64    `json:"height,omitempty"`
}

type TxSpec struct {
	Signer string `json:"signer"`
	Msg    string `json:"msg"`
	To     string `json:"to,omitempty"`
	Amount string `json:"amount,omitempty"`
	Gas    uint64 `json:"gas"`
	Key    string `json:"key,omitempty"`
	Value  int64  `json:"value,omitempty"`
	Field  string `json:"field,omitempty"`
}

func LoadFixture(dir, name string) (Fixture, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return Fixture{}, fmt.Errorf("read fixture %s: %w", name, err)
	}
	var f Fixture
	if err := yaml.Unmarshal(data, &f); err != nil {
		return Fixture{}, fmt.Errorf("parse fixture %s: %w", name, err)
	}
	return f, nil
}

func LoadCorpus(dir string) ([]Fixture, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("glob corpus %s: %w", dir, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no *.yaml fixtures found in %s", dir)
	}

	fixtures := make([]Fixture, 0, len(matches))
	for _, path := range matches {
		f, err := LoadFixture(filepath.Dir(path), filepath.Base(path))
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, f)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].Name < fixtures[j].Name })
	return fixtures, nil
}
