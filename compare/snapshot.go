package compare

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RangeMeta describes a snapshot corpus: the chain, app version, and the
// block height window this corpus covers.
type RangeMeta struct {
	ChainID    string `json:"chain_id"`
	AppVersion uint64 `json:"app_version"`
	Start      int64  `json:"start_height"`
	End        int64  `json:"end_height"`
	BondDenom  string `json:"bond_denom"`
}

// LoadRangeMeta reads and validates range.json from dir.
func LoadRangeMeta(dir string) (RangeMeta, error) {
	data, err := os.ReadFile(filepath.Join(dir, "range.json"))
	if err != nil {
		return RangeMeta{}, fmt.Errorf("read range.json: %w", err)
	}
	var m RangeMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return RangeMeta{}, fmt.Errorf("parse range.json: %w", err)
	}
	if m.Start <= 0 || m.End <= 0 || m.End < m.Start {
		return RangeMeta{}, fmt.Errorf("invalid height range [%d, %d]", m.Start, m.End)
	}
	return m, nil
}
