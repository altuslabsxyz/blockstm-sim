package compare

import (
	"fmt"

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
