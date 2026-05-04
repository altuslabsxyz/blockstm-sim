//go:build simharness_canary

package run

import (
	"fmt"

	appv1alpha1 "cosmossdk.io/api/cosmos/app/v1alpha1"
	"cosmossdk.io/core/appconfig"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/testutil/configurator"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	_ "github.com/altuslabsxyz/blockstm-sim/x/simcanary"
	simcanarykeeper "github.com/altuslabsxyz/blockstm-sim/x/simcanary/keeper"
	simcanarytypes "github.com/altuslabsxyz/blockstm-sim/x/simcanary/types"
)

// oracleCanaryKeeper is populated by depinject during oracle app setup in Init.
var oracleCanaryKeeper *simcanarykeeper.Keeper

func init() {
	extraModuleOpts = append(extraModuleOpts, simcanaryModule())

	extraTxBuilders["canary-map-set"] = buildCanaryMapSet
	extraTxBuilders["canary-map-read-write"] = buildCanaryMapReadAndWrite

	extraOracleOutputs = append(extraOracleOutputs, &oracleCanaryKeeper)

	extraOracleMutTrackers = func() []compare.MutationTracker {
		if oracleCanaryKeeper == nil {
			return nil
		}
		return []compare.MutationTracker{oracleCanaryKeeper}
	}
}

func simcanaryModule() configurator.ModuleOption {
	return func(config *configurator.Config) {
		config.ModuleConfigs[simcanarytypes.ModuleName] = &appv1alpha1.ModuleConfig{
			Name:   simcanarytypes.ModuleName,
			Config: appconfig.WrapAny(&simcanarytypes.Module{}),
		}
	}
}

func buildCanaryMapSet(spec compare.TxSpec, keys map[string]cryptotypes.PrivKey) ([]sdk.Msg, error) {
	fromKey, ok := keys[spec.Signer]
	if !ok {
		return nil, fmt.Errorf("unknown signer %q", spec.Signer)
	}
	return []sdk.Msg{
		&simcanarytypes.MsgCanaryMapSet{
			Sender: sdk.AccAddress(fromKey.PubKey().Address()).String(),
			Key:    spec.Key,
			Value:  spec.Value,
		},
	}, nil
}

func buildCanaryMapReadAndWrite(spec compare.TxSpec, keys map[string]cryptotypes.PrivKey) ([]sdk.Msg, error) {
	fromKey, ok := keys[spec.Signer]
	if !ok {
		return nil, fmt.Errorf("unknown signer %q", spec.Signer)
	}
	return []sdk.Msg{
		&simcanarytypes.MsgCanaryMapReadAndWrite{
			Sender: sdk.AccAddress(fromKey.PubKey().Address()).String(),
			Key:    spec.Key,
		},
	}, nil
}
