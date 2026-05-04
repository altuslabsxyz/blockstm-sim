//go:build sdk_hooks

package run

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestDeriveKey_Deterministic(t *testing.T) {
	k1 := DeriveKey("alice")
	k2 := DeriveKey("alice")

	require.True(t, bytes.Equal(k1.Bytes(), k2.Bytes()), "same name must yield same key")
	require.Equal(t,
		k1.PubKey().Address().String(),
		k2.PubKey().Address().String(),
		"same name must yield same address",
	)
}

func TestDeriveKey_DifferentNames(t *testing.T) {
	k1 := DeriveKey("alice")
	k2 := DeriveKey("bob")

	require.False(t, bytes.Equal(k1.Bytes(), k2.Bytes()), "different names must yield different keys")
}

func TestBankSendTxDecodable(t *testing.T) {
	fixture, err := compare.LoadFixture("../corpus/fixtures", "01-single-bank-send.yaml")
	require.NoError(t, err)

	exec := NewFixtureExecutor()
	require.NoError(t, exec.Init(fixture.Genesis))
	defer exec.Close()

	spec := fixture.Blocks[0].Txs[0]
	txBytes, err := exec.buildTx(spec)
	require.NoError(t, err)
	require.NotEmpty(t, txBytes)

	decoded, err := exec.txConfig.TxDecoder()(txBytes)
	require.NoError(t, err)

	msgs := decoded.GetMsgs()
	require.Len(t, msgs, 1)

	msgSend, ok := msgs[0].(*banktypes.MsgSend)
	require.True(t, ok, "expected MsgSend, got %T", msgs[0])
	require.Equal(t, "100stake", msgSend.Amount.String())
}
