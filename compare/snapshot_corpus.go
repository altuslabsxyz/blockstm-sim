package compare

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"os"

	cmttypes "github.com/cometbft/cometbft/types"
)

var _ CorpusStore = (*SnapshotCorpus)(nil)

// BlockLoader abstracts the CometBFT BlockStore for testability.
type BlockLoader interface {
	LoadBlock(height int64) *cmttypes.Block
	Close() error
}

// SnapshotCorpus supplies blocks and pre-state-directory metadata from a
// decompressed snapshot directory containing blockstore.db, application.db,
// and an optional range.json. When range.json is absent the block range,
// chain ID, and app version are inferred from blockstore.db automatically.
// The executor opens its own application.db handles from SnapshotDir; IAVL
// replay requires physically separate stores for oracle and probe, so the
// corpus deliberately does not hold application.db open.
type SnapshotCorpus struct {
	meta   RangeMeta
	loader BlockLoader
	dir    string
}

// NewSnapshotCorpusFromDir opens the snapshot at dir and returns a corpus.
// If range.json is present it is used as-is; if it is absent the block range,
// chain ID, and app version are inferred from blockstore.db automatically.
// Only blockstore.db is opened here; application.db is opened by the executor
// (SnapshotExecutor) so that oracle and probe each get their own physical DB.
// The caller must call Close() to release the blockstore database.
func NewSnapshotCorpusFromDir(dir string) (*SnapshotCorpus, error) {
	// Open blockstore first so we can infer RangeMeta when range.json is absent.
	bsDB, err := openBlockstoreDB(dir)
	if err != nil {
		return nil, fmt.Errorf("open blockstore.db: %w", err)
	}

	meta, err := LoadRangeMeta(dir)
	if errors.Is(err, os.ErrNotExist) {
		meta, err = inferRangeMetaFromBlockstore(bsDB)
	}
	if err != nil {
		_ = bsDB.Close()
		return nil, err
	}

	return &SnapshotCorpus{meta: meta, loader: bsDB, dir: dir}, nil
}

// NewSnapshotCorpus creates a corpus from pre-opened components.
// Intended for tests or custom wiring; the caller manages component lifecycle.
// dir is the snapshot directory containing application.db; it may be "" in
// tests that don't exercise the executor's DB-opening path.
func NewSnapshotCorpus(meta RangeMeta, loader BlockLoader, dir string) *SnapshotCorpus {
	return &SnapshotCorpus{meta: meta, loader: loader, dir: dir}
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
			rawTxs := make([][]byte, len(cmtBlock.Txs))
			for i, tx := range cmtBlock.Txs {
				rawTxs[i] = []byte(tx)
			}
			if !yield(Block{Height: h, RawTxs: rawTxs}, nil) {
				return
			}
		}
	}
}

func (sc *SnapshotCorpus) SnapshotDir() string  { return sc.dir }
func (sc *SnapshotCorpus) Meta() RangeMeta      { return sc.meta }
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
	return errors.Join(errs...)
}
