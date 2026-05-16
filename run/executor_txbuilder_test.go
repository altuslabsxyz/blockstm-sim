//go:build sdk_hooks

package run

import (
	"fmt"
	"testing"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestWithExtraTxBuilders_SetsField(t *testing.T) {
	fn := func(_ compare.TxSpec, _ map[string]cryptotypes.PrivKey) ([]sdk.Msg, error) {
		return nil, nil
	}
	e := NewFixtureExecutor(WithExtraTxBuilders(map[string]TxBuilderFn{"foo": fn}))
	require.Len(t, e.extraTxBuilders, 1)
	require.Contains(t, e.extraTxBuilders, "foo")
}

func TestMergedTxBuilders_InstanceOverridesPackage(t *testing.T) {
	orig := extraTxBuilders
	defer func() { extraTxBuilders = orig }()

	called := ""
	extraTxBuilders = map[string]TxBuilderFn{
		"shared": func(_ compare.TxSpec, _ map[string]cryptotypes.PrivKey) ([]sdk.Msg, error) {
			called = "package"
			return nil, nil
		},
	}
	e := NewFixtureExecutor(WithExtraTxBuilders(map[string]TxBuilderFn{
		"shared": func(_ compare.TxSpec, _ map[string]cryptotypes.PrivKey) ([]sdk.Msg, error) {
			called = "instance"
			return nil, nil
		},
	}))

	merged := e.mergedTxBuilders()
	_, _ = merged["shared"](compare.TxSpec{}, nil)
	require.Equal(t, "instance", called)
}

func TestMergedTxBuilders_PackageLevelPreserved(t *testing.T) {
	orig := extraTxBuilders
	defer func() { extraTxBuilders = orig }()

	extraTxBuilders = map[string]TxBuilderFn{
		"pkg-only": func(_ compare.TxSpec, _ map[string]cryptotypes.PrivKey) ([]sdk.Msg, error) {
			return nil, fmt.Errorf("pkg")
		},
	}
	e := NewFixtureExecutor(WithExtraTxBuilders(map[string]TxBuilderFn{
		"inst-only": func(_ compare.TxSpec, _ map[string]cryptotypes.PrivKey) ([]sdk.Msg, error) {
			return nil, fmt.Errorf("inst")
		},
	}))

	merged := e.mergedTxBuilders()
	require.Contains(t, merged, "pkg-only")
	require.Contains(t, merged, "inst-only")

	_, err := merged["pkg-only"](compare.TxSpec{}, nil)
	require.EqualError(t, err, "pkg")

	_, err = merged["inst-only"](compare.TxSpec{}, nil)
	require.EqualError(t, err, "inst")
}
