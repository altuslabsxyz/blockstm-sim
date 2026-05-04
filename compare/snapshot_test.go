package compare_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestLoadRangeMeta_OK(t *testing.T) {
	dir := t.TempDir()
	meta := compare.RangeMeta{
		ChainID:    "cosmoshub-4",
		AppVersion: 2,
		Start:      12345678,
		End:        12345778,
		BondDenom:  "uatom",
	}
	data, err := json.Marshal(meta)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "range.json"), data, 0o644))

	got, err := compare.LoadRangeMeta(dir)
	require.NoError(t, err)
	require.Equal(t, meta, got)
}

func TestLoadRangeMeta_InvalidRange(t *testing.T) {
	dir := t.TempDir()
	bad := `{"chain_id":"x","start_height":100,"end_height":50,"bond_denom":"stake"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "range.json"), []byte(bad), 0o644))

	_, err := compare.LoadRangeMeta(dir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid height range")
}

func TestLoadRangeMeta_Missing(t *testing.T) {
	_, err := compare.LoadRangeMeta(t.TempDir())
	require.Error(t, err)
}

func TestLoadRangeMeta_EmptyFields(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		json    string
		wantErr string
	}{
		{
			name:    "empty chain_id",
			json:    `{"chain_id":"","start_height":1,"end_height":10,"bond_denom":"stake"}`,
			wantErr: "chain_id is required",
		},
		{
			name:    "empty bond_denom",
			json:    `{"chain_id":"mychain","start_height":1,"end_height":10,"bond_denom":""}`,
			wantErr: "bond_denom is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "range.json"), []byte(tc.json), 0o644))
			_, err := compare.LoadRangeMeta(dir)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
