package simharness

import storetypes "cosmossdk.io/store/types"

var _ storetypes.CommitMultiStore = (*ParentMultiStore)(nil)

// ParentMultiStore wraps a CommitMultiStore and delegates
// CacheMultiStoreWithVersion to a parent store when set.
type ParentMultiStore struct {
	storetypes.CommitMultiStore
	parent storetypes.MultiStore
}

func NewParentMultiStore(cms storetypes.CommitMultiStore) *ParentMultiStore {
	return &ParentMultiStore{CommitMultiStore: cms}
}

func (p *ParentMultiStore) SetParent(parent storetypes.MultiStore) {
	p.parent = parent
}

func (p *ParentMultiStore) CacheMultiStoreWithVersion(version int64) (storetypes.CacheMultiStore, error) {
	if p.parent != nil {
		return p.parent.CacheMultiStoreWithVersion(version)
	}
	return p.CommitMultiStore.CacheMultiStoreWithVersion(version)
}
