package compare_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestLoadCorpus_SortsByName(t *testing.T) {
	fixtures, err := compare.LoadCorpus("testdata")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(fixtures), 2)

	for i := 1; i < len(fixtures); i++ {
		require.Less(t, fixtures[i-1].Name, fixtures[i].Name,
			"fixtures must be sorted by name")
	}
}

func TestLoadCorpus_EmptyDir(t *testing.T) {
	_, err := compare.LoadCorpus(t.TempDir())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no *.yaml fixtures found")
}

func TestFixture_IsCanary(t *testing.T) {
	f, err := compare.LoadFixture("testdata", "canary-01-always-diverge.yaml")
	require.NoError(t, err)
	require.True(t, f.IsCanary())
	require.Equal(t, compare.KindCanary, f.Kind)
}

func TestFixture_IsCanary_Default(t *testing.T) {
	f, err := compare.LoadFixture("testdata", "01-single-bank-send.yaml")
	require.NoError(t, err)
	require.False(t, f.IsCanary())
}
