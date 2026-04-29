package types

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/cosmos/gogoproto/proto"
)

// ---------------------------------------------------------------------------
// MsgCanaryMapSet — writes a key-value pair to the in-memory map
// ---------------------------------------------------------------------------

const msgCanaryMapSetProtoName = "simcanary.v1.MsgCanaryMapSet"

type MsgCanaryMapSet struct {
	Sender string `protobuf:"bytes,1,opt,name=sender,proto3" json:"sender,omitempty"`
	Key    string `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
	Value  int64  `protobuf:"varint,3,opt,name=value,proto3" json:"value,omitempty"`
}

var _ proto.Message = (*MsgCanaryMapSet)(nil)

func (*MsgCanaryMapSet) ProtoMessage()             {}
func (*MsgCanaryMapSet) Reset()                     {}
func (*MsgCanaryMapSet) Descriptor() ([]byte, []int) { return fileDescriptorBytes, []int{0} }
func (m *MsgCanaryMapSet) String() string {
	return fmt.Sprintf("MsgCanaryMapSet{sender:%s, key:%s, value:%d}", m.Sender, m.Key, m.Value)
}

func (m *MsgCanaryMapSet) Marshal() ([]byte, error) {
	size := m.Size()
	buf := make([]byte, size)
	n, err := m.MarshalToSizedBuffer(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (m *MsgCanaryMapSet) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgCanaryMapSet) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Value != 0 {
		i = encodeVarint(dAtA, i, uint64(m.Value))
		i--
		dAtA[i] = 0x18 // field 3, varint
	}
	if len(m.Key) > 0 {
		i -= len(m.Key)
		copy(dAtA[i:], m.Key)
		i = encodeVarint(dAtA, i, uint64(len(m.Key)))
		i--
		dAtA[i] = 0x12 // field 2, length-delimited
	}
	if len(m.Sender) > 0 {
		i -= len(m.Sender)
		copy(dAtA[i:], m.Sender)
		i = encodeVarint(dAtA, i, uint64(len(m.Sender)))
		i--
		dAtA[i] = 0x0a // field 1, length-delimited
	}
	return len(dAtA) - i, nil
}

func (m *MsgCanaryMapSet) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		fieldNum, wireType, n := decodeTag(dAtA[iNdEx:])
		if n < 0 {
			return io.ErrUnexpectedEOF
		}
		iNdEx += n
		switch fieldNum {
		case 1: // sender
			if wireType != 2 {
				return fmt.Errorf("unexpected wire type %d for field sender", wireType)
			}
			sLen, n := binary.Uvarint(dAtA[iNdEx:])
			if n <= 0 {
				return io.ErrUnexpectedEOF
			}
			iNdEx += n
			if iNdEx+int(sLen) > l {
				return io.ErrUnexpectedEOF
			}
			m.Sender = string(dAtA[iNdEx : iNdEx+int(sLen)])
			iNdEx += int(sLen)
		case 2: // key
			if wireType != 2 {
				return fmt.Errorf("unexpected wire type %d for field key", wireType)
			}
			sLen, n := binary.Uvarint(dAtA[iNdEx:])
			if n <= 0 {
				return io.ErrUnexpectedEOF
			}
			iNdEx += n
			if iNdEx+int(sLen) > l {
				return io.ErrUnexpectedEOF
			}
			m.Key = string(dAtA[iNdEx : iNdEx+int(sLen)])
			iNdEx += int(sLen)
		case 3: // value
			if wireType != 0 {
				return fmt.Errorf("unexpected wire type %d for field value", wireType)
			}
			v, n := binary.Uvarint(dAtA[iNdEx:])
			if n <= 0 {
				return io.ErrUnexpectedEOF
			}
			m.Value = int64(v)
			iNdEx += n
		default:
			iNdEx, _ = skipField(dAtA, iNdEx, wireType)
		}
	}
	return nil
}

func (m *MsgCanaryMapSet) Size() int {
	var n int
	if len(m.Sender) > 0 {
		n += 1 + sovLen(uint64(len(m.Sender))) + len(m.Sender)
	}
	if len(m.Key) > 0 {
		n += 1 + sovLen(uint64(len(m.Key))) + len(m.Key)
	}
	if m.Value != 0 {
		n += 1 + sovLen(uint64(m.Value))
	}
	return n
}

// ---------------------------------------------------------------------------
// MsgCanaryMapSetResponse
// ---------------------------------------------------------------------------

const msgCanaryMapSetResponseProtoName = "simcanary.v1.MsgCanaryMapSetResponse"

type MsgCanaryMapSetResponse struct{}

var _ proto.Message = (*MsgCanaryMapSetResponse)(nil)

func (*MsgCanaryMapSetResponse) ProtoMessage()             {}
func (*MsgCanaryMapSetResponse) Reset()                     {}
func (*MsgCanaryMapSetResponse) Descriptor() ([]byte, []int) { return fileDescriptorBytes, []int{1} }
func (*MsgCanaryMapSetResponse) String() string              { return "MsgCanaryMapSetResponse{}" }

func (*MsgCanaryMapSetResponse) Marshal() ([]byte, error)                { return nil, nil }
func (*MsgCanaryMapSetResponse) MarshalTo(dAtA []byte) (int, error)      { return 0, nil }
func (*MsgCanaryMapSetResponse) MarshalToSizedBuffer([]byte) (int, error) { return 0, nil }
func (*MsgCanaryMapSetResponse) Unmarshal([]byte) error                   { return nil }
func (*MsgCanaryMapSetResponse) Size() int                                { return 0 }

// ---------------------------------------------------------------------------
// MsgCanaryMapReadAndWrite — reads from map, writes to KVStore
// ---------------------------------------------------------------------------

const msgCanaryMapReadAndWriteProtoName = "simcanary.v1.MsgCanaryMapReadAndWrite"

type MsgCanaryMapReadAndWrite struct {
	Sender string `protobuf:"bytes,1,opt,name=sender,proto3" json:"sender,omitempty"`
	Key    string `protobuf:"bytes,2,opt,name=key,proto3" json:"key,omitempty"`
}

var _ proto.Message = (*MsgCanaryMapReadAndWrite)(nil)

func (*MsgCanaryMapReadAndWrite) ProtoMessage()             {}
func (*MsgCanaryMapReadAndWrite) Reset()                     {}
func (*MsgCanaryMapReadAndWrite) Descriptor() ([]byte, []int) { return fileDescriptorBytes, []int{2} }
func (m *MsgCanaryMapReadAndWrite) String() string {
	return fmt.Sprintf("MsgCanaryMapReadAndWrite{sender:%s, key:%s}", m.Sender, m.Key)
}

func (m *MsgCanaryMapReadAndWrite) Marshal() ([]byte, error) {
	size := m.Size()
	buf := make([]byte, size)
	n, err := m.MarshalToSizedBuffer(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (m *MsgCanaryMapReadAndWrite) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgCanaryMapReadAndWrite) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if len(m.Key) > 0 {
		i -= len(m.Key)
		copy(dAtA[i:], m.Key)
		i = encodeVarint(dAtA, i, uint64(len(m.Key)))
		i--
		dAtA[i] = 0x12 // field 2, length-delimited
	}
	if len(m.Sender) > 0 {
		i -= len(m.Sender)
		copy(dAtA[i:], m.Sender)
		i = encodeVarint(dAtA, i, uint64(len(m.Sender)))
		i--
		dAtA[i] = 0x0a // field 1, length-delimited
	}
	return len(dAtA) - i, nil
}

func (m *MsgCanaryMapReadAndWrite) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		fieldNum, wireType, n := decodeTag(dAtA[iNdEx:])
		if n < 0 {
			return io.ErrUnexpectedEOF
		}
		iNdEx += n
		switch fieldNum {
		case 1: // sender
			if wireType != 2 {
				return fmt.Errorf("unexpected wire type %d for field sender", wireType)
			}
			sLen, n := binary.Uvarint(dAtA[iNdEx:])
			if n <= 0 {
				return io.ErrUnexpectedEOF
			}
			iNdEx += n
			if iNdEx+int(sLen) > l {
				return io.ErrUnexpectedEOF
			}
			m.Sender = string(dAtA[iNdEx : iNdEx+int(sLen)])
			iNdEx += int(sLen)
		case 2: // key
			if wireType != 2 {
				return fmt.Errorf("unexpected wire type %d for field key", wireType)
			}
			sLen, n := binary.Uvarint(dAtA[iNdEx:])
			if n <= 0 {
				return io.ErrUnexpectedEOF
			}
			iNdEx += n
			if iNdEx+int(sLen) > l {
				return io.ErrUnexpectedEOF
			}
			m.Key = string(dAtA[iNdEx : iNdEx+int(sLen)])
			iNdEx += int(sLen)
		default:
			iNdEx, _ = skipField(dAtA, iNdEx, wireType)
		}
	}
	return nil
}

func (m *MsgCanaryMapReadAndWrite) Size() int {
	var n int
	if len(m.Sender) > 0 {
		n += 1 + sovLen(uint64(len(m.Sender))) + len(m.Sender)
	}
	if len(m.Key) > 0 {
		n += 1 + sovLen(uint64(len(m.Key))) + len(m.Key)
	}
	return n
}

// ---------------------------------------------------------------------------
// MsgCanaryMapReadAndWriteResponse
// ---------------------------------------------------------------------------

const msgCanaryMapReadAndWriteResponseProtoName = "simcanary.v1.MsgCanaryMapReadAndWriteResponse"

type MsgCanaryMapReadAndWriteResponse struct {
	ObservedValue int64 `protobuf:"varint,1,opt,name=observed_value,proto3" json:"observed_value,omitempty"`
}

var _ proto.Message = (*MsgCanaryMapReadAndWriteResponse)(nil)

func (*MsgCanaryMapReadAndWriteResponse) ProtoMessage()             {}
func (*MsgCanaryMapReadAndWriteResponse) Reset()                     {}
func (*MsgCanaryMapReadAndWriteResponse) Descriptor() ([]byte, []int) { return fileDescriptorBytes, []int{3} }
func (m *MsgCanaryMapReadAndWriteResponse) String() string {
	return fmt.Sprintf("MsgCanaryMapReadAndWriteResponse{observed_value:%d}", m.ObservedValue)
}

func (m *MsgCanaryMapReadAndWriteResponse) Marshal() ([]byte, error) {
	size := m.Size()
	buf := make([]byte, size)
	n, err := m.MarshalToSizedBuffer(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (m *MsgCanaryMapReadAndWriteResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgCanaryMapReadAndWriteResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.ObservedValue != 0 {
		i = encodeVarint(dAtA, i, uint64(m.ObservedValue))
		i--
		dAtA[i] = 0x08 // field 1, varint
	}
	return len(dAtA) - i, nil
}

func (m *MsgCanaryMapReadAndWriteResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		fieldNum, wireType, n := decodeTag(dAtA[iNdEx:])
		if n < 0 {
			return io.ErrUnexpectedEOF
		}
		iNdEx += n
		switch fieldNum {
		case 1: // observed_value
			if wireType != 0 {
				return fmt.Errorf("unexpected wire type %d for field observed_value", wireType)
			}
			v, n := binary.Uvarint(dAtA[iNdEx:])
			if n <= 0 {
				return io.ErrUnexpectedEOF
			}
			m.ObservedValue = int64(v)
			iNdEx += n
		default:
			iNdEx, _ = skipField(dAtA, iNdEx, wireType)
		}
	}
	return nil
}

func (m *MsgCanaryMapReadAndWriteResponse) Size() int {
	var n int
	if m.ObservedValue != 0 {
		n += 1 + sovLen(uint64(m.ObservedValue))
	}
	return n
}

// ---------------------------------------------------------------------------
// Proto type registration
// ---------------------------------------------------------------------------

func init() {
	proto.RegisterType((*MsgCanaryMapSet)(nil), msgCanaryMapSetProtoName)
	proto.RegisterType((*MsgCanaryMapSetResponse)(nil), msgCanaryMapSetResponseProtoName)
	proto.RegisterType((*MsgCanaryMapReadAndWrite)(nil), msgCanaryMapReadAndWriteProtoName)
	proto.RegisterType((*MsgCanaryMapReadAndWriteResponse)(nil), msgCanaryMapReadAndWriteResponseProtoName)
}

// ---------------------------------------------------------------------------
// Protobuf wire-format helpers (matching gogo codegen output)
// ---------------------------------------------------------------------------

func encodeVarint(dAtA []byte, offset int, v uint64) int {
	offset--
	dAtA[offset] = uint8(v)
	if v >= 1<<7 {
		dAtA[offset] |= 0x80
		offset--
		dAtA[offset] = uint8(v>>7) & 0x7f
		if v >= 1<<14 {
			dAtA[offset] |= 0x80
			offset--
			dAtA[offset] = uint8(v>>14) & 0x7f
			if v >= 1<<21 {
				dAtA[offset] |= 0x80
				offset--
				dAtA[offset] = uint8(v>>21) & 0x7f
				if v >= 1<<28 {
					dAtA[offset] |= 0x80
					offset--
					dAtA[offset] = uint8(v>>28) & 0x7f
					if v >= 1<<35 {
						dAtA[offset] |= 0x80
						offset--
						dAtA[offset] = uint8(v>>35) & 0x7f
						if v >= 1<<42 {
							dAtA[offset] |= 0x80
							offset--
							dAtA[offset] = uint8(v>>42) & 0x7f
							if v >= 1<<49 {
								dAtA[offset] |= 0x80
								offset--
								dAtA[offset] = uint8(v>>49) & 0x7f
								if v >= 1<<56 {
									dAtA[offset] |= 0x80
									offset--
									dAtA[offset] = uint8(v>>56) & 0x7f
									if v >= 1<<63 {
										dAtA[offset] |= 0x80
										offset--
										dAtA[offset] = 1
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return offset
}

func sovLen(x uint64) int {
	n := 0
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}

func decodeTag(b []byte) (fieldNum int, wireType int, n int) {
	v, n := binary.Uvarint(b)
	if n <= 0 {
		return 0, 0, -1
	}
	return int(v >> 3), int(v & 0x7), n
}

func skipField(dAtA []byte, iNdEx int, wireType int) (int, error) {
	switch wireType {
	case 0: // varint
		for iNdEx < len(dAtA) {
			if dAtA[iNdEx]&0x80 == 0 {
				return iNdEx + 1, nil
			}
			iNdEx++
		}
		return len(dAtA), io.ErrUnexpectedEOF
	case 1: // 64-bit
		return iNdEx + 8, nil
	case 2: // length-delimited
		sLen, n := binary.Uvarint(dAtA[iNdEx:])
		if n <= 0 {
			return iNdEx, io.ErrUnexpectedEOF
		}
		return iNdEx + n + int(sLen), nil
	case 5: // 32-bit
		return iNdEx + 4, nil
	default:
		return iNdEx, fmt.Errorf("unknown wire type %d", wireType)
	}
}
