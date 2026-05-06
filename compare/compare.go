package compare

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"
)

type Finalizer interface {
	FinalizeBlock(*abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error)
}

type Input struct {
	Oracle                Finalizer
	Probe                 Finalizer
	Block                 *abci.RequestFinalizeBlock
	OracleWriteSets       WriteSetProvider
	ProbeWriteSets        WriteSetProvider
	OracleMutations       MutationProvider
	BlockContextMutations BlockContextMutationProvider
	// PostOracleHook is called immediately after oracle FinalizeBlock succeeds,
	// before the probe runs. Use this to capture oracle-side state that must be
	// snapshotted after oracle execution but before any further state changes.
	PostOracleHook func()
}

func Run(input Input) (*Result, error) {
	oracleRes, err := input.Oracle.FinalizeBlock(input.Block)
	if err != nil {
		return nil, fmt.Errorf("oracle FinalizeBlock: %w", err)
	}
	if input.PostOracleHook != nil {
		input.PostOracleHook()
	}

	probeRes, err := input.Probe.FinalizeBlock(input.Block)
	if err != nil {
		return nil, fmt.Errorf("probe FinalizeBlock: %w", err)
	}

	height := input.Block.Height
	result := &Result{Height: height}
	var findings []Finding

	if !bytes.Equal(oracleRes.AppHash, probeRes.AppHash) {
		findings = append(findings, NewFinding(
			height, DimAppHash, -1, 0,
			hex.EncodeToString(oracleRes.AppHash),
			hex.EncodeToString(probeRes.AppHash),
		))
	}

	txCount := min(len(oracleRes.TxResults), len(probeRes.TxResults))
	for i := 0; i < txCount; i++ {
		if oracleRes.TxResults[i].Code != probeRes.TxResults[i].Code {
			findings = append(findings, NewFinding(
				height, DimErrorCode, i, 0,
				fmt.Sprintf("%d", oracleRes.TxResults[i].Code),
				fmt.Sprintf("%d", probeRes.TxResults[i].Code),
			))
		}
	}

	if input.OracleWriteSets != nil && input.ProbeWriteSets != nil {
		for i := 0; i < txCount; i++ {
			oWS := input.OracleWriteSets.TxWriteSet(i)
			pWS := input.ProbeWriteSets.TxWriteSet(i)
			if !EqualStrSlice(oWS, pWS) {
				findings = append(findings, NewFinding(
					height, DimWriteSet, i, 0,
					FormatWriteSet(oWS),
					FormatWriteSet(pWS),
				))
			}
		}
	}

	if input.OracleMutations != nil {
		// Use the larger of txCount and len(Block.Txs) so that block-level
		// mutations (stored at txIndex=0) are always read, even when a runner
		// returns fewer TxResults than the number of submitted transactions.
		for i := 0; i < max(txCount, len(input.Block.Txs)); i++ {
			for _, m := range input.OracleMutations.TxMutations(i) {
				findings = append(findings, NewFinding(
					height, DimOutOfKVStore, i, 0,
					fmt.Sprintf("%s:%s", m.Tracker, hex.EncodeToString(m.Before)),
					fmt.Sprintf("%s:%s", m.Tracker, hex.EncodeToString(m.After)),
				))
			}
		}
	}

	if input.BlockContextMutations != nil {
		for _, m := range input.BlockContextMutations.BlockContextMutations() {
			findings = append(findings, NewFinding(
				height, DimBlockContext, m.WriterTx, 0,
				fmt.Sprintf("field=%s;before=%s;readers=%s", m.Field, m.Before, joinInts(m.ReaderTxs)),
				fmt.Sprintf("field=%s;after=%s;writer=tx%d", m.Field, m.After, m.WriterTx),
			))
		}
	}

	if len(findings) > 0 {
		result.Verdict = Divergence
		result.Findings = findings
	} else {
		result.Verdict = Match
	}

	return result, nil
}

// EqualStrSlice reports whether a and b contain the same strings in the same
// order.
func EqualStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// FormatWriteSet returns a human-readable summary of a write-set key list,
// truncating to the first 5 keys when the slice is longer.
func FormatWriteSet(keys []string) string {
	if len(keys) == 0 {
		return "(empty)"
	}
	const maxDisplay = 5
	if len(keys) <= maxDisplay {
		return strings.Join(keys, ",")
	}
	return strings.Join(keys[:maxDisplay], ",") + fmt.Sprintf(",…(%d total)", len(keys))
}
