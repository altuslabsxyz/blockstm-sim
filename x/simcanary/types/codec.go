package types

import (
	"context"

	"github.com/cosmos/gogoproto/proto"
	grpc1 "github.com/cosmos/gogoproto/grpc"
	"google.golang.org/grpc"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// MsgServer is the server API for the simcanary Msg service.
type MsgServer interface {
	MapSet(context.Context, *MsgCanaryMapSet) (*MsgCanaryMapSetResponse, error)
	MapReadAndWrite(context.Context, *MsgCanaryMapReadAndWrite) (*MsgCanaryMapReadAndWriteResponse, error)
}

func RegisterMsgServer(s grpc1.Server, srv MsgServer) {
	s.RegisterService(&_Msg_serviceDesc, srv)
}

func _Msg_MapSet_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgCanaryMapSet)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).MapSet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/simcanary.v1.Msg/MapSet",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).MapSet(ctx, req.(*MsgCanaryMapSet))
	}
	return interceptor(ctx, in, info, handler)
}

func _Msg_MapReadAndWrite_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgCanaryMapReadAndWrite)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).MapReadAndWrite(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/simcanary.v1.Msg/MapReadAndWrite",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).MapReadAndWrite(ctx, req.(*MsgCanaryMapReadAndWrite))
	}
	return interceptor(ctx, in, info, handler)
}

var _ MsgServer = (*UnimplementedMsgServer)(nil)

type UnimplementedMsgServer struct{}

func (*UnimplementedMsgServer) MapSet(context.Context, *MsgCanaryMapSet) (*MsgCanaryMapSetResponse, error) {
	return nil, nil
}

func (*UnimplementedMsgServer) MapReadAndWrite(context.Context, *MsgCanaryMapReadAndWrite) (*MsgCanaryMapReadAndWriteResponse, error) {
	return nil, nil
}

var _Msg_serviceDesc = grpc.ServiceDesc{
	ServiceName: "simcanary.v1.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "MapSet",
			Handler:    _Msg_MapSet_Handler,
		},
		{
			MethodName: "MapReadAndWrite",
			Handler:    _Msg_MapReadAndWrite_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: txProtoFile,
}

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCanaryMapSet{},
		&MsgCanaryMapReadAndWrite{},
	)
	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

// Ensure message types satisfy sdk.Msg (which is proto.Message in v0.50).
var (
	_ sdk.Msg       = (*MsgCanaryMapSet)(nil)
	_ sdk.Msg       = (*MsgCanaryMapReadAndWrite)(nil)
	_ proto.Message = (*MsgCanaryMapSetResponse)(nil)
	_ proto.Message = (*MsgCanaryMapReadAndWriteResponse)(nil)
)
