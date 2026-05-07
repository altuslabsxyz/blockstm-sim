//go:build sdk_hooks

package run

import (
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp/txnrunner"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// newSTMRunnerForProbe builds a *txnrunner.STMRunner for the given probeConfig.
// Probe with seed=0 gets a plain STM runner; seed>0 gets a perturbed runner.
func newSTMRunnerForProbe(cfg probeConfig, txDecoder sdk.TxDecoder, storeKeys []storetypes.StoreKey) *txnrunner.STMRunner {
	if cfg.seed == 0 {
		return txnrunner.NewSTMRunner(txDecoder, storeKeys, cfg.workers, false, nil)
	}
	return txnrunner.NewSTMRunner(txDecoder, storeKeys, cfg.workers, false, nil,
		txnrunner.WithPerturbHook(cfg.seed))
}
