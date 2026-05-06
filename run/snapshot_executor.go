package run

import (
	"fmt"

	dbm "github.com/cosmos/cosmos-db"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// SnapshotExecutor executes blocks against real mainnet pre-state.
//
// NOTE: InitFromState requires SDK hook support for versioned multistore
// snapshots. This executor is a placeholder and returns an error until that
// support is available in the configured SDK.
type SnapshotExecutor struct{}

func NewSnapshotExecutor() *SnapshotExecutor { return &SnapshotExecutor{} }

// Init is not supported; SnapshotExecutor requires state-based initialisation.
func (e *SnapshotExecutor) Init(_ compare.GenesisSpec) error {
	return fmt.Errorf("SnapshotExecutor requires InitFromState with SDK hook support")
}

// InitFromState loads oracle and probe apps from preStateDB.
func (e *SnapshotExecutor) InitFromState(_ dbm.DB) error {
	return fmt.Errorf("InitFromState not yet implemented for the configured SDK")
}

// RunBlock executes a block containing raw transaction bytes.
func (e *SnapshotExecutor) RunBlock(_ compare.BlockSpec, _ int64) (*compare.Result, error) {
	return nil, fmt.Errorf("SnapshotExecutor not initialised")
}

func (e *SnapshotExecutor) Close() {}
