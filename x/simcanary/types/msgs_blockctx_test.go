package types_test

import (
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/x/simcanary/types"
)

func TestMsgCanaryBlockContextSet_RoundTrip(t *testing.T) {
	orig := &types.MsgCanaryBlockContextSet{
		Sender: "cosmos1abc",
		Field:  "height",
		Value:  "999",
	}
	b, err := proto.Marshal(orig)
	require.NoError(t, err)

	got := &types.MsgCanaryBlockContextSet{}
	require.NoError(t, proto.Unmarshal(b, got))
	require.Equal(t, orig.Sender, got.Sender)
	require.Equal(t, orig.Field, got.Field)
	require.Equal(t, orig.Value, got.Value)
}

func TestMsgCanaryBlockContextRead_RoundTrip(t *testing.T) {
	orig := &types.MsgCanaryBlockContextRead{
		Sender: "cosmos1xyz",
		Field:  "chain_id",
	}
	b, err := proto.Marshal(orig)
	require.NoError(t, err)

	got := &types.MsgCanaryBlockContextRead{}
	require.NoError(t, proto.Unmarshal(b, got))
	require.Equal(t, orig.Sender, got.Sender)
	require.Equal(t, orig.Field, got.Field)
}

func TestMsgCanaryBlockContextReadResponse_RoundTrip(t *testing.T) {
	orig := &types.MsgCanaryBlockContextReadResponse{Value: "1"}
	b, err := proto.Marshal(orig)
	require.NoError(t, err)

	got := &types.MsgCanaryBlockContextReadResponse{}
	require.NoError(t, proto.Unmarshal(b, got))
	require.Equal(t, orig.Value, got.Value)
}
