package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/x/simcanary/keeper"
)

type stubBlockCtxWriter struct {
	reads  map[string]int
	writes map[string]string
}

func newStubWriter() *stubBlockCtxWriter {
	return &stubBlockCtxWriter{
		reads:  make(map[string]int),
		writes: make(map[string]string),
	}
}

func (s *stubBlockCtxWriter) ReadField(name string) string {
	s.reads[name]++
	return "stub-" + name
}

func (s *stubBlockCtxWriter) WriteField(name, value string) {
	s.writes[name] = value
}

func TestKeeper_BlockContextWrite(t *testing.T) {
	k := keeper.NewKeeper(nil)
	w := newStubWriter()
	k.SetBlockContextWriter(w)

	k.WriteBlockCtxField("height", "999")
	require.Equal(t, "999", w.writes["height"])
}

func TestKeeper_BlockContextRead(t *testing.T) {
	k := keeper.NewKeeper(nil)
	w := newStubWriter()
	k.SetBlockContextWriter(w)

	val := k.ReadBlockCtxField("chain_id")
	require.Equal(t, "stub-chain_id", val)
	require.Equal(t, 1, w.reads["chain_id"])
}

func TestKeeper_BlockContextNilWriter(t *testing.T) {
	k := keeper.NewKeeper(nil)
	k.WriteBlockCtxField("height", "999")
	got := k.ReadBlockCtxField("height")
	require.Empty(t, got)
}
