package detect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModuleFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"x/bank/keeper/msg_server.go", "bank"},
		{"x/staking/types/validator.go", "staking"},
		{"x/auth/tx/config/config.go", "auth"},
		{"baseapp/abci.go", "baseapp"},
		{"server/start.go", "server"},
		{"store/rootmulti/store.go", "store"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ModuleFromPath(tt.path)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCategoryString(t *testing.T) {
	require.Equal(t, "time", string(CatTime))
	require.Equal(t, "rand", string(CatRand))
	require.Equal(t, "io", string(CatIO))
}
