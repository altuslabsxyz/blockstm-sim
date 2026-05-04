//go:build sdk_hooks && simharness_canary

package run

import (
	"fmt"

	appv1alpha1 "cosmossdk.io/api/cosmos/app/v1alpha1"
	"cosmossdk.io/core/appconfig"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/configurator"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/altuslabsxyz/blockstm-sim/compare"
	"github.com/altuslabsxyz/blockstm-sim/coverage"
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

	coverage.Register("canary-map-set", coverage.Entry{
		Key:       "canary-map-set",
		Module:    "simcanary",
		MsgType:   "MsgCanaryMapSet",
		HandlerFn: "MapSet",
	})
	coverage.Register("canary-map-read-write", coverage.Entry{
		Key:       "canary-map-read-write",
		Module:    "simcanary",
		MsgType:   "MsgCanaryMapReadAndWrite",
		HandlerFn: "MapReadAndWrite",
	})
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
