package compare

import (
	"bytes"
	"encoding/hex"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
)

type Finalizer interface {
	FinalizeBlock(*abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error)
}

type Input struct {
	Oracle Finalizer
	Probe  Finalizer
	Block  *abci.RequestFinalizeBlock
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

	if bytes.Equal(oracleRes.AppHash, probeRes.AppHash) {
		result.Verdict = Match
		return result, nil
	}

	result.Verdict = Divergence
	result.Findings = []Finding{
		NewFinding(
			height,
			DimAppHash,
			-1,
			0,
			hex.EncodeToString(oracleRes.AppHash),
			hex.EncodeToString(probeRes.AppHash),
		),
	}

	return result, nil
}
