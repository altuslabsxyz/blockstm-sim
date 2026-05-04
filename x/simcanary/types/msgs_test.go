package types_test

import (
	"testing"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/x/simcanary/types"
)

func TestMsgCanaryMapSet_MarshalRoundTrip(t *testing.T) {
	msg := &types.MsgCanaryMapSet{
		Sender: "cosmos1abc",
		Key:    "k",
		Value:  42,
	}

	bz, err := proto.Marshal(msg)
	require.NoError(t, err)
	t.Logf("marshaled bytes: %x (len=%d)", bz, len(bz))

	out := &types.MsgCanaryMapSet{}
	err = proto.Unmarshal(bz, out)
	require.NoError(t, err)
	require.Equal(t, msg.Sender, out.Sender)
	require.Equal(t, msg.Key, out.Key)
	require.Equal(t, msg.Value, out.Value)
}

func TestMsgCanaryMapReadAndWrite_MarshalRoundTrip(t *testing.T) {
	msg := &types.MsgCanaryMapReadAndWrite{
		Sender: "cosmos1xyz",
		Key:    "mykey",
	}

	bz, err := proto.Marshal(msg)
	require.NoError(t, err)
	t.Logf("marshaled bytes: %x (len=%d)", bz, len(bz))

	out := &types.MsgCanaryMapReadAndWrite{}
	err = proto.Unmarshal(bz, out)
	require.NoError(t, err)
	require.Equal(t, msg.Sender, out.Sender)
	require.Equal(t, msg.Key, out.Key)
}

func TestMsgCanaryMapSetResponse_MarshalRoundTrip(t *testing.T) {
	msg := &types.MsgCanaryMapSetResponse{}
	bz, err := proto.Marshal(msg)
	require.NoError(t, err)
	require.Len(t, bz, 0)

	out := &types.MsgCanaryMapSetResponse{}
	require.NoError(t, proto.Unmarshal(bz, out))
}

func TestMsgCanaryMapReadAndWriteResponse_MarshalRoundTrip(t *testing.T) {
	msg := &types.MsgCanaryMapReadAndWriteResponse{ObservedValue: 42}
	bz, err := proto.Marshal(msg)
	require.NoError(t, err)

	out := &types.MsgCanaryMapReadAndWriteResponse{}
	require.NoError(t, proto.Unmarshal(bz, out))
	require.Equal(t, int64(42), out.ObservedValue)
}

func TestMsgTypeRegistration(t *testing.T) {
	typ := proto.MessageType("simcanary.v1.MsgCanaryMapSet")
	require.NotNil(t, typ, "MsgCanaryMapSet should be registered")

	typ = proto.MessageType("simcanary.v1.MsgCanaryMapReadAndWrite")
	require.NotNil(t, typ, "MsgCanaryMapReadAndWrite should be registered")

	typ = proto.MessageType("simcanary.v1.MsgCanaryBlockContextSet")
	require.NotNil(t, typ, "MsgCanaryBlockContextSet should be registered")

	typ = proto.MessageType("simcanary.v1.MsgCanaryBlockContextRead")
	require.NotNil(t, typ, "MsgCanaryBlockContextRead should be registered")

	name := proto.MessageName(&types.MsgCanaryMapSet{})
	require.Equal(t, "simcanary.v1.MsgCanaryMapSet", name)
}

func TestFileDescriptorRegistered(t *testing.T) {
	fd := proto.FileDescriptor("simcanary/v1/tx.proto")
	require.NotNil(t, fd, "file descriptor should be registered")
}
