package compare

import (
	"testing"

	cmttypes "github.com/cometbft/cometbft/types"
	cmtversion "github.com/cometbft/cometbft/proto/tendermint/version"
	"github.com/stretchr/testify/require"
)

// stubRangeSource is an in-process blockRangeSource for testing inference
// without a real LevelDB blockstore.
type stubRangeSource struct {
	base, height int64
	blocks       map[int64]*cmttypes.Block
}

func (s *stubRangeSource) Base() int64   { return s.base }
func (s *stubRangeSource) Height() int64 { return s.height }
func (s *stubRangeSource) LoadBlock(h int64) *cmttypes.Block {
	if s.blocks == nil {
		return nil
	}
	return s.blocks[h]
}

func makeVersionedBlock(height int64, chainID string, appVersion uint64) *cmttypes.Block {
	return &cmttypes.Block{
		Header: cmttypes.Header{
			Height:  height,
			ChainID: chainID,
			Version: cmtversion.Consensus{App: appVersion},
		},
	}
}

func TestInferRangeMetaFromBlockstore_OK(t *testing.T) {
	src := &stubRangeSource{
		base:   10,
		height: 100,
		blocks: map[int64]*cmttypes.Block{
			10: makeVersionedBlock(10, "testnet-1", 3),
		},
	}

	meta, err := inferRangeMetaFromBlockstore(src)

	require.NoError(t, err)
	require.Equal(t, int64(10), meta.Start)
	require.Equal(t, int64(100), meta.End)
	require.Equal(t, "testnet-1", meta.ChainID)
	require.Equal(t, uint64(3), meta.AppVersion)
	require.Equal(t, "", meta.BondDenom)
}

func TestInferRangeMetaFromBlockstore_EmptyStore(t *testing.T) {
	src := &stubRangeSource{base: 0, height: 0}

	_, err := inferRangeMetaFromBlockstore(src)

	require.Error(t, err)
	require.Contains(t, err.Error(), "blockstore is empty")
}

func TestInferRangeMetaFromBlockstore_MissingBaseBlock(t *testing.T) {
	src := &stubRangeSource{
		base:   5,
		height: 50,
		blocks: map[int64]*cmttypes.Block{}, // base block absent
	}

	_, err := inferRangeMetaFromBlockstore(src)

	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot load block at base height 5")
}
