package run

import (
	"fmt"

	dbm "github.com/cosmos/cosmos-db"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// SnapshotExecutor executes blocks against real mainnet pre-state.
//
// NOTE: InitFromState requires CacheMultiStoreWithVersion, which is blocked
// on stable-sdk PR-3b "MultiStore decorator". This executor is a placeholder
// and will return an error if Init or InitFromState is called until that PR merges.
type SnapshotExecutor struct{}

func NewSnapshotExecutor() *SnapshotExecutor { return &SnapshotExecutor{} }

// Init is not supported; SnapshotExecutor requires state-based initialisation.
func (e *SnapshotExecutor) Init(_ compare.GenesisSpec) error {
	return fmt.Errorf("SnapshotExecutor requires InitFromState (pending stable-sdk PR-3b)")
}

// InitFromState loads oracle and probe apps from preStateDB.
// Blocked on stable-sdk PR-3b: CacheMultiStoreWithVersion panics until merged.
func (e *SnapshotExecutor) InitFromState(_ dbm.DB) error {
	return fmt.Errorf("InitFromState not yet implemented: waiting for stable-sdk PR-3b")
}

// RunBlock executes a block containing raw transaction bytes.
func (e *SnapshotExecutor) RunBlock(_ compare.BlockSpec, _ int64) (*compare.Result, error) {
	return nil, fmt.Errorf("SnapshotExecutor not initialised")
}

func (e *SnapshotExecutor) Close() {}
