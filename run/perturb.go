package run

import (
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/altuslabsxyz/blockstm-sim/sdkhook"
)

// newSTMRunnerForProbe builds an STMRunner for the given probeConfig via the
// registered sdkhook factory. A seed of 0 produces no perturbation (baseline).
func newSTMRunnerForProbe(cfg probeConfig, txDecoder sdk.TxDecoder, storeKeys []storetypes.StoreKey) sdkhook.STMRunner {
	return sdkhook.NewSTMRunner(txDecoder, storeKeys, cfg.workers, cfg.seed)
}
