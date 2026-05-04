//go:build sdk_hooks

package run

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

// mockWS is a trivial WriteSetProvider for testing.
type mockWS struct {
	sets map[int][]string
}

func (m *mockWS) TxWriteSet(i int) []string { return m.sets[i] }

// TestRepeatCompareResponses_AllMatch verifies that identical responses produce
// no findings.
func TestRepeatCompareResponses_AllMatch(t *testing.T) {
	hash := []byte{0xde, 0xad, 0xbe, 0xef}
	left := &abci.ResponseFinalizeBlock{
		AppHash: hash,
		TxResults: []*abci.ExecTxResult{
			{Code: 0},
			{Code: 0},
		},
	}
	right := &abci.ResponseFinalizeBlock{
		AppHash: hash,
		TxResults: []*abci.ExecTxResult{
			{Code: 0},
			{Code: 0},
		},
	}

	ws := &mockWS{sets: map[int][]string{
		0: {"key/a"},
		1: {"key/b"},
	}}

	findings := repeatCompareResponses(1, left, right, ws, ws, 2, 2)
	assert.Empty(t, findings, "identical responses must produce no findings")
}

// TestRepeatCompareResponses_AppHashMismatch verifies that an AppHash mismatch
// produces a DimAppHash finding with the expected ProbeIndex.
func TestRepeatCompareResponses_AppHashMismatch(t *testing.T) {
	left := &abci.ResponseFinalizeBlock{
		AppHash:   []byte{0x01},
		TxResults: []*abci.ExecTxResult{},
	}
	right := &abci.ResponseFinalizeBlock{
		AppHash:   []byte{0x02},
		TxResults: []*abci.ExecTxResult{},
	}

	findings := repeatCompareResponses(1, left, right, nil, nil, 0, 2)
	require.Len(t, findings, 1)
	f := findings[0]
	assert.Equal(t, compare.DimAppHash, f.Dimension)
	assert.Equal(t, -1, f.TxIndex)
	assert.Equal(t, 2, f.ProbeIndex)
}

// TestRepeatCompareResponses_ErrorCodeMismatch verifies that a mismatched
// error code on tx 1 produces a DimErrorCode finding.
func TestRepeatCompareResponses_ErrorCodeMismatch(t *testing.T) {
	hash := []byte{0xaa}
	left := &abci.ResponseFinalizeBlock{
		AppHash: hash,
		TxResults: []*abci.ExecTxResult{
			{Code: 0},
			{Code: 3},
		},
	}
	right := &abci.ResponseFinalizeBlock{
		AppHash: hash,
		TxResults: []*abci.ExecTxResult{
			{Code: 0},
			{Code: 7},
		},
	}

	findings := repeatCompareResponses(1, left, right, nil, nil, 2, 1)
	require.Len(t, findings, 1)
	f := findings[0]
	assert.Equal(t, compare.DimErrorCode, f.Dimension)
	assert.Equal(t, 1, f.TxIndex)
	assert.Equal(t, "3", f.Oracle)
	assert.Equal(t, "7", f.Probe)
}

// TestRepeatCompareResponses_WriteSetMismatch verifies that a mismatched write
// set on tx 0 produces a DimWriteSet finding.
func TestRepeatCompareResponses_WriteSetMismatch(t *testing.T) {
	hash := []byte{0xbb}
	left := &abci.ResponseFinalizeBlock{
		AppHash: hash,
		TxResults: []*abci.ExecTxResult{
			{Code: 0},
		},
	}
	right := &abci.ResponseFinalizeBlock{
		AppHash: hash,
		TxResults: []*abci.ExecTxResult{
			{Code: 0},
		},
	}

	leftWS := &mockWS{sets: map[int][]string{0: {"store/a", "store/b"}}}
	rightWS := &mockWS{sets: map[int][]string{0: {"store/a", "store/c"}}}

	findings := repeatCompareResponses(1, left, right, leftWS, rightWS, 1, 0)
	require.Len(t, findings, 1)
	f := findings[0]
	assert.Equal(t, compare.DimWriteSet, f.Dimension)
	assert.Equal(t, 0, f.TxIndex)
}
