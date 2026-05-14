package compare

import (
	"fmt"
	"os"

	cmtdb "github.com/cometbft/cometbft-db"
	cmtstore "github.com/cometbft/cometbft/store"
	cmttypes "github.com/cometbft/cometbft/types"
)

// cmtBlockStore wraps *cmtstore.BlockStore to satisfy BlockLoader.
type cmtBlockStore struct {
	db cmtdb.DB
	bs *cmtstore.BlockStore
}

func openBlockstoreDB(dir string) (*cmtBlockStore, error) {
	db, err := cmtdb.NewGoLevelDB("blockstore", dir)
	if err != nil {
		return nil, fmt.Errorf("leveldb open: %w", err)
	}
	bs := cmtstore.NewBlockStore(db)
	return &cmtBlockStore{db: db, bs: bs}, nil
}

func (c *cmtBlockStore) LoadBlock(height int64) *cmttypes.Block {
	return c.bs.LoadBlock(height)
}

func (c *cmtBlockStore) Close() error {
	return c.db.Close()
}

func (c *cmtBlockStore) Base() int64   { return c.bs.Base() }
func (c *cmtBlockStore) Height() int64 { return c.bs.Height() }

// blockRangeSource is the minimal interface needed to infer RangeMeta from a
// blockstore without coupling inferRangeMetaFromBlockstore to *cmtBlockStore.
type blockRangeSource interface {
	Base() int64
	Height() int64
	LoadBlock(int64) *cmttypes.Block
}

// inferRangeMetaFromBlockstore derives RangeMeta from a live blockstore when
// range.json is absent. BondDenom is left empty because snapshot mode replays
// raw transactions and never generates new ones.
func inferRangeMetaFromBlockstore(src blockRangeSource) (RangeMeta, error) {
	height := src.Height()
	if height == 0 {
		return RangeMeta{}, fmt.Errorf("blockstore is empty")
	}
	base := src.Base()
	block := src.LoadBlock(base)
	if block == nil {
		return RangeMeta{}, fmt.Errorf("cannot load block at base height %d", base)
	}
	meta := RangeMeta{
		ChainID:    block.ChainID,
		AppVersion: block.Version.App,
		Start:      base,
		End:        height,
	}
	fmt.Fprintf(os.Stderr,
		"range.json not found; auto-detected: chain=%s start=%d end=%d\n",
		meta.ChainID, meta.Start, meta.End)
	return meta, nil
}
