package types

import (
	"github.com/cosmos/gogoproto/proto"
)

const moduleConfigProtoName = "simcanary.module.v1.Module"

// Module is the hand-written module config proto message for depinject.
// It has no fields — the module needs no configuration.
type Module struct{}

var _ proto.Message = (*Module)(nil)

func (*Module) Reset()         {}
func (*Module) String() string { return moduleConfigProtoName }
func (*Module) ProtoMessage()  {}

func (*Module) Marshal() ([]byte, error)                { return nil, nil }
func (*Module) MarshalTo(dAtA []byte) (int, error)      { return 0, nil }
func (*Module) MarshalToSizedBuffer([]byte) (int, error) { return 0, nil }
func (*Module) Unmarshal([]byte) error                   { return nil }
func (*Module) Size() int                                { return 0 }

func init() {
	proto.RegisterType((*Module)(nil), moduleConfigProtoName)
}
