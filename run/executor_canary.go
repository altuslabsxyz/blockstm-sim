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
	"github.com/altuslabsxyz/blockstm-sim/coverage"
	_ "github.com/altuslabsxyz/blockstm-sim/x/simcanary"
	simcanarytypes "github.com/altuslabsxyz/blockstm-sim/x/simcanary/types"
)

func init() {
	extraModuleOpts = append(extraModuleOpts, simcanaryModule())

	extraTxBuilders["canary-map-set"] = buildCanaryMapSet
	extraTxBuilders["canary-map-read-write"] = buildCanaryMapReadAndWrite

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
