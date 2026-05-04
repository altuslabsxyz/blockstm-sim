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
	Oracle          Finalizer
	Probe           Finalizer
	Block           *abci.RequestFinalizeBlock
	OracleWriteSets WriteSetProvider
	ProbeWriteSets  WriteSetProvider
}

func Run(input Input) (*Result, error) {
	oracleRes, err := input.Oracle.FinalizeBlock(input.Block)
	if err != nil {
		return nil, fmt.Errorf("oracle FinalizeBlock: %w", err)
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
			if !equalStrSlice(oWS, pWS) {
				findings = append(findings, NewFinding(
					height, DimWriteSet, i, 0,
					formatWriteSet(oWS),
					formatWriteSet(pWS),
				))
			}
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

func equalStrSlice(a, b []string) bool {
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

func formatWriteSet(keys []string) string {
	if len(keys) == 0 {
		return "(empty)"
	}
	const maxDisplay = 5
	if len(keys) <= maxDisplay {
		return strings.Join(keys, ",")
	}
	return strings.Join(keys[:maxDisplay], ",") + fmt.Sprintf(",…(%d total)", len(keys))
}
