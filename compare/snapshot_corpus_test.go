package compare_test

import (
	"context"
	"testing"

	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// stubBlockLoader is an in-memory BlockLoader for tests.
type stubBlockLoader struct {
	blocks map[int64]*cmttypes.Block
}

func (s *stubBlockLoader) LoadBlock(height int64) *cmttypes.Block { return s.blocks[height] }
func (s *stubBlockLoader) Close() error                          { return nil }

func makeStubBlock(height int64, rawTxs ...[]byte) *cmttypes.Block {
	txs := make(cmttypes.Txs, len(rawTxs))
	for i, tx := range rawTxs {
		txs[i] = tx
	}
	return &cmttypes.Block{
		Header: cmttypes.Header{Height: height},
		Data:   cmttypes.Data{Txs: txs},
	}
}

func newTestCorpus(meta compare.RangeMeta, blocks map[int64]*cmttypes.Block) *compare.SnapshotCorpus {
	loader := &stubBlockLoader{blocks: blocks}
	return compare.NewSnapshotCorpus(meta, loader, "")
}

func TestSnapshotCorpus_Iter(t *testing.T) {
	meta := compare.RangeMeta{
		ChainID:   "test-chain",
		Start:     10,
		End:       12,
		BondDenom: "stake",
	}
	blocks := map[int64]*cmttypes.Block{
		10: makeStubBlock(10, []byte("tx-a")),
		11: makeStubBlock(11, []byte("tx-b"), []byte("tx-c")),
		12: makeStubBlock(12),
	}
	sc := newTestCorpus(meta, blocks)
	defer func() { _ = sc.Close() }()

	var got []compare.Block
	for block, err := range sc.Iter(context.Background()) {
		require.NoError(t, err)
		got = append(got, block)
	}

	require.Len(t, got, 3)
	require.Equal(t, int64(10), got[0].Height)
	require.Equal(t, [][]byte{[]byte("tx-a")}, got[0].RawTxs)
	require.Equal(t, int64(11), got[1].Height)
	require.Len(t, got[1].RawTxs, 2)
	require.Equal(t, int64(12), got[2].Height)
	require.Empty(t, got[2].RawTxs)
}

func TestSnapshotCorpus_IterCancel(t *testing.T) {
	meta := compare.RangeMeta{ChainID: "x", Start: 1, End: 5, BondDenom: "stake"}
	blocks := map[int64]*cmttypes.Block{
		1: makeStubBlock(1, []byte("tx")),
		2: makeStubBlock(2, []byte("tx")),
	}
	sc := newTestCorpus(meta, blocks)
	defer func() { _ = sc.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var collected []compare.Block
	var iterErr error
	for block, err := range sc.Iter(ctx) {
		if err != nil {
			iterErr = err
			break
		}
		collected = append(collected, block)
		cancel()
	}

	require.ErrorIs(t, iterErr, context.Canceled)
	require.Len(t, collected, 1)
}

func TestSnapshotCorpus_MissingBlock(t *testing.T) {
	meta := compare.RangeMeta{ChainID: "x", Start: 1, End: 2, BondDenom: "stake"}
	blocks := map[int64]*cmttypes.Block{
		1: makeStubBlock(1),
		// height 2 is intentionally absent
	}
	sc := newTestCorpus(meta, blocks)
	defer func() { _ = sc.Close() }()

	var iterErr error
	for _, err := range sc.Iter(context.Background()) {
		if err != nil {
			iterErr = err
			break
		}
	}
	require.Error(t, iterErr)
	require.Contains(t, iterErr.Error(), "block 2 not found")
}

func TestSnapshotCorpus_Metadata(t *testing.T) {
	meta := compare.RangeMeta{
		ChainID:    "cosmoshub-4",
		AppVersion: 2,
		Start:      100,
		End:        199,
		BondDenom:  "uatom",
	}
	sc := newTestCorpus(meta, nil)
	defer func() { _ = sc.Close() }()

	require.Equal(t, "cosmoshub-4", sc.Name())
	require.Equal(t, "uatom", sc.BondDenom())
	require.Equal(t, 100, sc.BlockCount())
	require.False(t, sc.IsCanary())
	require.Empty(t, sc.SnapshotDir())
	require.Equal(t, meta, sc.Meta())
	require.Equal(t, compare.GenesisSpec{}, sc.Genesis())
}
