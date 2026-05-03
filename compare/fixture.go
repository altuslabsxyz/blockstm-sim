package compare

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

type Fixture struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Genesis     GenesisSpec `json:"genesis"`
	Blocks      []BlockSpec `json:"blocks"`
}

type GenesisSpec struct {
	Accounts map[string]AccountSpec `json:"accounts"`
}

type AccountSpec struct {
	Balance string `json:"balance"`
}

type BlockSpec struct {
	Txs []TxSpec `json:"txs"`
}

type TxSpec struct {
	Signer string `json:"signer"`
	Msg    string `json:"msg"`
	To     string `json:"to"`
	Amount string `json:"amount"`
	Gas    uint64 `json:"gas"`
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
