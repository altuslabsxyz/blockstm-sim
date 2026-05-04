package compare

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type Dimension string

const (
	DimAppHash   Dimension = "app_hash"
	DimErrorCode Dimension = "error_code"
	DimWriteSet  Dimension = "write_set"
)

type Finding struct {
	ID      string
	Height  int64
	TxIndex int // -1 for block-level findings

	// ProbeIndex identifies which probe diverged.
	// ProbeIndex=0: F1 finding (sequential oracle vs probe[0]).
	// ProbeIndex=i (i>0): F2 finding (probe[0] baseline vs probe[i] in a repeat-run check).
	ProbeIndex int

	Dimension Dimension

	// Oracle holds the reference value: the sequential oracle result for ProbeIndex=0,
	// or probe[0]'s result for ProbeIndex>0.
	Oracle string
	// Probe holds the diverging value from the probe identified by ProbeIndex.
	Probe string
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
