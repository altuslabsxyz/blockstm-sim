package types

import (
	"bytes"
	"compress/gzip"

	gogoproto "github.com/cosmos/gogoproto/proto"
	proto2 "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	msgv1 "cosmossdk.io/api/cosmos/msg/v1"
)

const txProtoFile = "simcanary/v1/tx.proto"

// fileDescriptorBytes holds the gzipped FileDescriptorProto.
// Shared by proto.RegisterFile and each type's Descriptor() method.
var fileDescriptorBytes []byte

func init() {
	fileDescriptorBytes = buildFileDescriptor()
	gogoproto.RegisterFile(txProtoFile, fileDescriptorBytes)
}

func buildFileDescriptor() []byte {
	svcOpts := &descriptorpb.ServiceOptions{}
	proto2.SetExtension(svcOpts, msgv1.E_Service, true)

	signerOpts := &descriptorpb.MessageOptions{}
	proto2.SetExtension(signerOpts, msgv1.E_Signer, []string{"sender"})

	fd := &descriptorpb.FileDescriptorProto{
		Name:       strPtr(txProtoFile),
		Package:    strPtr("simcanary.v1"),
		Syntax:     strPtr("proto3"),
		Dependency: []string{"cosmos/msg/v1/msg.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			msgDescriptorWithOpts("MsgCanaryMapSet", signerOpts,
				fieldString(1, "sender"),
				fieldString(2, "key"),
				fieldInt64(3, "value"),
			),
			msgDescriptor("MsgCanaryMapSetResponse"),
			msgDescriptorWithOpts("MsgCanaryMapReadAndWrite", signerOpts,
				fieldString(1, "sender"),
				fieldString(2, "key"),
			),
			msgDescriptor("MsgCanaryMapReadAndWriteResponse",
				fieldInt64(1, "observed_value"),
			),
		},
		Service: []*descriptorpb.ServiceDescriptorProto{{
			Name:    strPtr("Msg"),
			Options: svcOpts,
			Method: []*descriptorpb.MethodDescriptorProto{
				{
					Name:       strPtr("MapSet"),
					InputType:  strPtr(".simcanary.v1.MsgCanaryMapSet"),
					OutputType: strPtr(".simcanary.v1.MsgCanaryMapSetResponse"),
				},
				{
					Name:       strPtr("MapReadAndWrite"),
					InputType:  strPtr(".simcanary.v1.MsgCanaryMapReadAndWrite"),
					OutputType: strPtr(".simcanary.v1.MsgCanaryMapReadAndWriteResponse"),
				},
			},
		}},
	}

	raw, err := proto2.Marshal(fd)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(raw); err != nil {
		panic(err)
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func strPtr(s string) *string { return &s }

func msgDescriptor(name string, fields ...*descriptorpb.FieldDescriptorProto) *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{
		Name:  strPtr(name),
		Field: fields,
	}
}

func msgDescriptorWithOpts(name string, opts *descriptorpb.MessageOptions, fields ...*descriptorpb.FieldDescriptorProto) *descriptorpb.DescriptorProto {
	return &descriptorpb.DescriptorProto{
		Name:    strPtr(name),
		Field:   fields,
		Options: opts,
	}
}

func fieldString(num int32, name string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:     strPtr(name),
		Number:   int32Ptr(num),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		JsonName: strPtr(name),
	}
}

func fieldInt64(num int32, name string) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:     strPtr(name),
		Number:   int32Ptr(num),
		Type:     descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
		Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		JsonName: strPtr(name),
	}
}

func int32Ptr(i int32) *int32 { return &i }
