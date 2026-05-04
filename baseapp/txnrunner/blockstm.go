package txnrunner

import (
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/internal/blockstm"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.TxRunner = &STMRunner{}

// STMRunner is a public export of the internal implementation of a BlockSTM TxRunner.
type STMRunner = blockstm.STMRunner

// STMRunnerOption configures an STMRunner.
type STMRunnerOption = blockstm.STMRunnerOption

// WithPerturbHook returns an STMRunnerOption that injects deterministic micro-delays
// at scheduler phase boundaries. The seed makes the delay pattern reproducible.
func WithPerturbHook(seed int64) STMRunnerOption {
	return blockstm.WithSchedulerOptions(blockstm.WithHook(blockstm.NewPerturbHook(seed)))
}

// NewSTMRunner is a public export of the internal implementation of a BlockSTM TxRunner constructor.
func NewSTMRunner(
	txDecoder sdk.TxDecoder,
	stores []storetypes.StoreKey,
	workers int,
	estimate bool,
	coinDenom func(storetypes.MultiStore) string,
	opts ...STMRunnerOption,
) *STMRunner {
	return blockstm.NewSTMRunner(txDecoder, stores, workers, estimate, coinDenom, opts...)
}
