package compare

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type Dimension string

const (
	DimAppHash      Dimension = "app_hash"
	DimErrorCode    Dimension = "error_code"
	DimWriteSet     Dimension = "write_set"
	DimOutOfKVStore Dimension = "out_of_kvstore"
	DimBlockContext Dimension = "block_context"
)

type Finding struct {
	ID         string
	Height     int64
	TxIndex    int // -1 for block-level findings
	ProbeIndex int
	Dimension  Dimension
	Oracle     string // hex-encoded value
	Probe      string // hex-encoded value
}

func NewFinding(height int64, dim Dimension, txIdx, probeIdx int, oracle, probe string) Finding {
	return Finding{
		ID:         FindingID(height, dim, txIdx, probeIdx),
		Height:     height,
		TxIndex:    txIdx,
		ProbeIndex: probeIdx,
		Dimension:  dim,
		Oracle:     oracle,
		Probe:      probe,
	}
}

func FindingID(height int64, dim Dimension, txIdx, probeIdx int) string {
	raw := fmt.Sprintf("%d|%s|%d|%d", height, dim, txIdx, probeIdx)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:6])
}
