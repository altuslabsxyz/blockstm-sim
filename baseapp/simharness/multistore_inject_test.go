package simharness_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp/simharness"
)

type stubCMS struct {
	storetypes.CommitMultiStore
	versionCalled int64
}

func (s *stubCMS) CacheMultiStoreWithVersion(version int64) (storetypes.CacheMultiStore, error) {
	s.versionCalled = version
	return nil, fmt.Errorf("stub cms: version %d", version)
}

type stubParent struct {
	storetypes.MultiStore
	versionCalled int64
}

func (s *stubParent) CacheMultiStoreWithVersion(version int64) (storetypes.CacheMultiStore, error) {
	s.versionCalled = version
	return nil, fmt.Errorf("stub parent: version %d", version)
}

func TestParentMultiStore_WithoutParent(t *testing.T) {
	cms := &stubCMS{}
	pms := simharness.NewParentMultiStore(cms)

	_, err := pms.CacheMultiStoreWithVersion(42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stub cms")
	assert.Equal(t, int64(42), cms.versionCalled)
}

func TestParentMultiStore_WithParent(t *testing.T) {
	cms := &stubCMS{}
	parent := &stubParent{}
	pms := simharness.NewParentMultiStore(cms)
	pms.SetParent(parent)

	_, err := pms.CacheMultiStoreWithVersion(100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stub parent")
	assert.Equal(t, int64(100), parent.versionCalled)
	assert.Equal(t, int64(0), cms.versionCalled)
}
