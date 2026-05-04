package compare

import (
	"context"
	"errors"
	"fmt"
	"iter"

	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
)

// BlockLoader abstracts the CometBFT BlockStore for testability.
type BlockLoader interface {
	LoadBlock(height int64) *cmttypes.Block
	Close() error
}

// SnapshotCorpus supplies blocks and pre-state from a decompressed snapshot
// directory containing blockstore.db, application.db, and range.json.
type SnapshotCorpus struct {
	meta   RangeMeta
	loader BlockLoader
	appDB  dbm.DB
}

// NewSnapshotCorpusFromDir opens the snapshot at dir and returns a corpus.
// The caller must call Close() to release the underlying databases.
func NewSnapshotCorpusFromDir(dir string) (*SnapshotCorpus, error) {
	meta, err := LoadRangeMeta(dir)
	if err != nil {
		return nil, err
	}

	bsDB, err := openBlockstoreDB(dir)
	if err != nil {
		return nil, fmt.Errorf("open blockstore.db: %w", err)
	}

	appDB, err := dbm.NewGoLevelDB("application", dir, nil)
	if err != nil {
		_ = bsDB.Close()
		return nil, fmt.Errorf("open application.db: %w", err)
	}

	return &SnapshotCorpus{meta: meta, loader: bsDB, appDB: appDB}, nil
}

// NewSnapshotCorpus creates a corpus from pre-opened components.
// Intended for tests or custom wiring; the caller manages component lifecycle.
func NewSnapshotCorpus(meta RangeMeta, loader BlockLoader, appDB dbm.DB) *SnapshotCorpus {
	return &SnapshotCorpus{meta: meta, loader: loader, appDB: appDB}
}

// Iter yields one Block per height in [meta.Start, meta.End], each carrying
// the actual chain height in Block.Height and raw tx bytes in Block.RawTxs.
func (sc *SnapshotCorpus) Iter(ctx context.Context) iter.Seq2[Block, error] {
	return func(yield func(Block, error) bool) {
		for h := sc.meta.Start; h <= sc.meta.End; h++ {
			if ctx.Err() != nil {
				yield(Block{}, ctx.Err())
				return
			}
			cmtBlock := sc.loader.LoadBlock(h)
			if cmtBlock == nil {
				yield(Block{}, fmt.Errorf("block %d not found in blockstore", h))
				return
			}
			rawTxs := make([][]byte, len(cmtBlock.Data.Txs))
			for i, tx := range cmtBlock.Data.Txs {
				rawTxs[i] = []byte(tx)
			}
			if !yield(Block{Height: h, RawTxs: rawTxs}, nil) {
				return
			}
		}
	}
}

func (sc *SnapshotCorpus) PreStateDB() dbm.DB   { return sc.appDB }
func (sc *SnapshotCorpus) BondDenom() string    { return sc.meta.BondDenom }
func (sc *SnapshotCorpus) Name() string         { return sc.meta.ChainID }
func (sc *SnapshotCorpus) IsCanary() bool       { return false }
func (sc *SnapshotCorpus) Genesis() GenesisSpec { return GenesisSpec{} }
func (sc *SnapshotCorpus) BlockCount() int      { return int(sc.meta.End-sc.meta.Start) + 1 }

func (sc *SnapshotCorpus) Close() error {
	var errs []error
	if sc.loader != nil {
		if err := sc.loader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close blockstore: %w", err))
		}
	}
	if sc.appDB != nil {
		if err := sc.appDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close application.db: %w", err))
		}
	}
	return errors.Join(errs...)
}
