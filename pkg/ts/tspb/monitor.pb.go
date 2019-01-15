// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: ts/tspb/monitor.proto

package tspb

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"

import encoding_binary "encoding/binary"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type RollupRes int32

const (
	RollupRes_invalid RollupRes = 0
	RollupRes_oneM    RollupRes = 1
	RollupRes_tenM    RollupRes = 2
	RollupRes_oneH    RollupRes = 3
	RollupRes_twelveH RollupRes = 4
	RollupRes_oneD    RollupRes = 5
	RollupRes_oneW    RollupRes = 6
	RollupRes_stopper RollupRes = 7
)

var RollupRes_name = map[int32]string{
	0: "invalid",
	1: "oneM",
	2: "tenM",
	3: "oneH",
	4: "twelveH",
	5: "oneD",
	6: "oneW",
	7: "stopper",
}
var RollupRes_value = map[string]int32{
	"invalid": 0,
	"oneM":    1,
	"tenM":    2,
	"oneH":    3,
	"twelveH": 4,
	"oneD":    5,
	"oneW":    6,
	"stopper": 7,
}

func (x RollupRes) String() string {
	return proto.EnumName(RollupRes_name, int32(x))
}
func (RollupRes) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_monitor_d79b1dadf16ddced, []int{0}
}

type RollupDatapoint struct {
	First                float64  `protobuf:"fixed64,1,opt,name=first,proto3" json:"first,omitempty"`
	Last                 float64  `protobuf:"fixed64,2,opt,name=last,proto3" json:"last,omitempty"`
	Min                  float64  `protobuf:"fixed64,3,opt,name=min,proto3" json:"min,omitempty"`
	Max                  float64  `protobuf:"fixed64,4,opt,name=max,proto3" json:"max,omitempty"`
	Sum                  float64  `protobuf:"fixed64,5,opt,name=sum,proto3" json:"sum,omitempty"`
	Count                uint32   `protobuf:"varint,6,opt,name=count,proto3" json:"count,omitempty"`
	Variance             float64  `protobuf:"fixed64,7,opt,name=variance,proto3" json:"variance,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RollupDatapoint) Reset()         { *m = RollupDatapoint{} }
func (m *RollupDatapoint) String() string { return proto.CompactTextString(m) }
func (*RollupDatapoint) ProtoMessage()    {}
func (*RollupDatapoint) Descriptor() ([]byte, []int) {
	return fileDescriptor_monitor_d79b1dadf16ddced, []int{0}
}
func (m *RollupDatapoint) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *RollupDatapoint) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	b = b[:cap(b)]
	n, err := m.MarshalTo(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}
func (dst *RollupDatapoint) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RollupDatapoint.Merge(dst, src)
}
func (m *RollupDatapoint) XXX_Size() int {
	return m.Size()
}
func (m *RollupDatapoint) XXX_DiscardUnknown() {
	xxx_messageInfo_RollupDatapoint.DiscardUnknown(m)
}

var xxx_messageInfo_RollupDatapoint proto.InternalMessageInfo

type RollupCol struct {
	TimestampNanos       int64             `protobuf:"varint,1,opt,name=timestampNanos,proto3" json:"timestampNanos,omitempty"`
	Values               []RollupDatapoint `protobuf:"bytes,2,rep,name=values,proto3" json:"values"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *RollupCol) Reset()         { *m = RollupCol{} }
func (m *RollupCol) String() string { return proto.CompactTextString(m) }
func (*RollupCol) ProtoMessage()    {}
func (*RollupCol) Descriptor() ([]byte, []int) {
	return fileDescriptor_monitor_d79b1dadf16ddced, []int{1}
}
func (m *RollupCol) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *RollupCol) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	b = b[:cap(b)]
	n, err := m.MarshalTo(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}
func (dst *RollupCol) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RollupCol.Merge(dst, src)
}
func (m *RollupCol) XXX_Size() int {
	return m.Size()
}
func (m *RollupCol) XXX_DiscardUnknown() {
	xxx_messageInfo_RollupCol.DiscardUnknown(m)
}

var xxx_messageInfo_RollupCol proto.InternalMessageInfo

type NodeRollupDatapoint struct {
	NodeID               string     `protobuf:"bytes,1,opt,name=nodeID,proto3" json:"nodeID,omitempty"`
	Rollup               *RollupCol `protobuf:"bytes,2,opt,name=rollup,proto3" json:"rollup,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *NodeRollupDatapoint) Reset()         { *m = NodeRollupDatapoint{} }
func (m *NodeRollupDatapoint) String() string { return proto.CompactTextString(m) }
func (*NodeRollupDatapoint) ProtoMessage()    {}
func (*NodeRollupDatapoint) Descriptor() ([]byte, []int) {
	return fileDescriptor_monitor_d79b1dadf16ddced, []int{2}
}
func (m *NodeRollupDatapoint) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *NodeRollupDatapoint) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	b = b[:cap(b)]
	n, err := m.MarshalTo(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}
func (dst *NodeRollupDatapoint) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeRollupDatapoint.Merge(dst, src)
}
func (m *NodeRollupDatapoint) XXX_Size() int {
	return m.Size()
}
func (m *NodeRollupDatapoint) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeRollupDatapoint.DiscardUnknown(m)
}

var xxx_messageInfo_NodeRollupDatapoint proto.InternalMessageInfo

func init() {
	proto.RegisterType((*RollupDatapoint)(nil), "tspb.RollupDatapoint")
	proto.RegisterType((*RollupCol)(nil), "tspb.RollupCol")
	proto.RegisterType((*NodeRollupDatapoint)(nil), "tspb.NodeRollupDatapoint")
	proto.RegisterEnum("tspb.RollupRes", RollupRes_name, RollupRes_value)
}
func (m *RollupDatapoint) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *RollupDatapoint) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.First != 0 {
		dAtA[i] = 0x9
		i++
		encoding_binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.First))))
		i += 8
	}
	if m.Last != 0 {
		dAtA[i] = 0x11
		i++
		encoding_binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.Last))))
		i += 8
	}
	if m.Min != 0 {
		dAtA[i] = 0x19
		i++
		encoding_binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.Min))))
		i += 8
	}
	if m.Max != 0 {
		dAtA[i] = 0x21
		i++
		encoding_binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.Max))))
		i += 8
	}
	if m.Sum != 0 {
		dAtA[i] = 0x29
		i++
		encoding_binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.Sum))))
		i += 8
	}
	if m.Count != 0 {
		dAtA[i] = 0x30
		i++
		i = encodeVarintMonitor(dAtA, i, uint64(m.Count))
	}
	if m.Variance != 0 {
		dAtA[i] = 0x39
		i++
		encoding_binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.Variance))))
		i += 8
	}
	return i, nil
}

func (m *RollupCol) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *RollupCol) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.TimestampNanos != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintMonitor(dAtA, i, uint64(m.TimestampNanos))
	}
	if len(m.Values) > 0 {
		for _, msg := range m.Values {
			dAtA[i] = 0x12
			i++
			i = encodeVarintMonitor(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	return i, nil
}

func (m *NodeRollupDatapoint) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *NodeRollupDatapoint) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.NodeID) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintMonitor(dAtA, i, uint64(len(m.NodeID)))
		i += copy(dAtA[i:], m.NodeID)
	}
	if m.Rollup != nil {
		dAtA[i] = 0x12
		i++
		i = encodeVarintMonitor(dAtA, i, uint64(m.Rollup.Size()))
		n1, err := m.Rollup.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	return i, nil
}

func encodeVarintMonitor(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *RollupDatapoint) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.First != 0 {
		n += 9
	}
	if m.Last != 0 {
		n += 9
	}
	if m.Min != 0 {
		n += 9
	}
	if m.Max != 0 {
		n += 9
	}
	if m.Sum != 0 {
		n += 9
	}
	if m.Count != 0 {
		n += 1 + sovMonitor(uint64(m.Count))
	}
	if m.Variance != 0 {
		n += 9
	}
	return n
}

func (m *RollupCol) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.TimestampNanos != 0 {
		n += 1 + sovMonitor(uint64(m.TimestampNanos))
	}
	if len(m.Values) > 0 {
		for _, e := range m.Values {
			l = e.Size()
			n += 1 + l + sovMonitor(uint64(l))
		}
	}
	return n
}

func (m *NodeRollupDatapoint) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.NodeID)
	if l > 0 {
		n += 1 + l + sovMonitor(uint64(l))
	}
	if m.Rollup != nil {
		l = m.Rollup.Size()
		n += 1 + l + sovMonitor(uint64(l))
	}
	return n
}

func sovMonitor(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozMonitor(x uint64) (n int) {
	return sovMonitor(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *RollupDatapoint) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMonitor
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: RollupDatapoint: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: RollupDatapoint: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field First", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(encoding_binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.First = float64(math.Float64frombits(v))
		case 2:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Last", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(encoding_binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Last = float64(math.Float64frombits(v))
		case 3:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Min", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(encoding_binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Min = float64(math.Float64frombits(v))
		case 4:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Max", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(encoding_binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Max = float64(math.Float64frombits(v))
		case 5:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sum", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(encoding_binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Sum = float64(math.Float64frombits(v))
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Count", wireType)
			}
			m.Count = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMonitor
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Count |= (uint32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 7:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Variance", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(encoding_binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Variance = float64(math.Float64frombits(v))
		default:
			iNdEx = preIndex
			skippy, err := skipMonitor(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMonitor
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *RollupCol) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMonitor
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: RollupCol: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: RollupCol: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field TimestampNanos", wireType)
			}
			m.TimestampNanos = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMonitor
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.TimestampNanos |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Values", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMonitor
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMonitor
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Values = append(m.Values, RollupDatapoint{})
			if err := m.Values[len(m.Values)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMonitor(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMonitor
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *NodeRollupDatapoint) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMonitor
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: NodeRollupDatapoint: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: NodeRollupDatapoint: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field NodeID", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMonitor
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthMonitor
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.NodeID = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Rollup", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMonitor
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthMonitor
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Rollup == nil {
				m.Rollup = &RollupCol{}
			}
			if err := m.Rollup.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMonitor(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthMonitor
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipMonitor(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowMonitor
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMonitor
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowMonitor
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthMonitor
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowMonitor
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipMonitor(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthMonitor = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowMonitor   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("ts/tspb/monitor.proto", fileDescriptor_monitor_d79b1dadf16ddced) }

var fileDescriptor_monitor_d79b1dadf16ddced = []byte{
	// 376 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x5c, 0x91, 0xbf, 0xae, 0xd3, 0x30,
	0x14, 0xc6, 0xe3, 0x26, 0x4d, 0xef, 0x75, 0x05, 0xd7, 0x32, 0xf7, 0x22, 0xeb, 0x0e, 0xa1, 0xea,
	0x00, 0x15, 0x43, 0x2b, 0xdd, 0xbe, 0x41, 0xdb, 0xa1, 0x0c, 0xed, 0xe0, 0x01, 0x66, 0xb7, 0x31,
	0xc5, 0x92, 0xe3, 0x13, 0xc5, 0x4e, 0xe8, 0x63, 0xf0, 0x0e, 0xbc, 0x4c, 0x47, 0x46, 0x26, 0x04,
	0xe1, 0x45, 0x90, 0x9d, 0x80, 0x50, 0xb7, 0xef, 0xfb, 0x9d, 0x4f, 0x3e, 0x7f, 0x8c, 0x1f, 0x9c,
	0x5d, 0x38, 0x5b, 0x1e, 0x16, 0x05, 0x18, 0xe5, 0xa0, 0x9a, 0x97, 0x15, 0x38, 0xa0, 0x89, 0x67,
	0x8f, 0xf7, 0x27, 0x38, 0x41, 0x00, 0x0b, 0xaf, 0xba, 0xda, 0xf4, 0x2b, 0xc2, 0x77, 0x1c, 0xb4,
	0xae, 0xcb, 0x8d, 0x70, 0xa2, 0x04, 0x65, 0x1c, 0xbd, 0xc7, 0xc3, 0x8f, 0xaa, 0xb2, 0x8e, 0xa1,
	0x09, 0x9a, 0x21, 0xde, 0x19, 0x4a, 0x71, 0xa2, 0x85, 0x75, 0x6c, 0x10, 0x60, 0xd0, 0x94, 0xe0,
	0xb8, 0x50, 0x86, 0xc5, 0x01, 0x79, 0x19, 0x88, 0x38, 0xb3, 0xa4, 0x27, 0xe2, 0xec, 0x89, 0xad,
	0x0b, 0x36, 0xec, 0x88, 0xad, 0x0b, 0xff, 0xfe, 0x11, 0x6a, 0xe3, 0x58, 0x3a, 0x41, 0xb3, 0x67,
	0xbc, 0x33, 0xf4, 0x11, 0xdf, 0x34, 0xa2, 0x52, 0xc2, 0x1c, 0x25, 0x1b, 0x85, 0xf0, 0x3f, 0x3f,
	0xfd, 0x84, 0x6f, 0xbb, 0x21, 0xd7, 0xa0, 0xe9, 0x6b, 0xfc, 0xdc, 0xa9, 0x42, 0x5a, 0x27, 0x8a,
	0x72, 0x2f, 0x0c, 0xd8, 0x30, 0x67, 0xcc, 0xaf, 0x28, 0x5d, 0xe2, 0xb4, 0x11, 0xba, 0x96, 0x96,
	0x0d, 0x26, 0xf1, 0x6c, 0xfc, 0xf4, 0x30, 0xf7, 0x77, 0x98, 0x5f, 0x6d, 0xbb, 0x4a, 0x2e, 0x3f,
	0x5e, 0x45, 0xbc, 0x8f, 0x4e, 0xdf, 0xe3, 0x17, 0x7b, 0xc8, 0xe5, 0xf5, 0x49, 0x5e, 0xe2, 0xd4,
	0x40, 0x2e, 0xdf, 0x6d, 0x42, 0xaf, 0x5b, 0xde, 0x3b, 0xfa, 0x06, 0xa7, 0x55, 0x88, 0x86, 0xb3,
	0x8c, 0x9f, 0xee, 0xfe, 0xef, 0xb1, 0x06, 0xcd, 0xfb, 0xf2, 0xdb, 0xfc, 0xef, 0x06, 0x5c, 0x5a,
	0x3a, 0xc6, 0x23, 0x65, 0x1a, 0xa1, 0x55, 0x4e, 0x22, 0x7a, 0x83, 0x13, 0x30, 0x72, 0x47, 0x90,
	0x57, 0x4e, 0x9a, 0x1d, 0x19, 0xf4, 0x6c, 0x4b, 0x62, 0x1f, 0x75, 0x9f, 0xa5, 0x6e, 0xe4, 0x96,
	0x24, 0x3d, 0xde, 0x90, 0x61, 0xaf, 0x3e, 0x90, 0xd4, 0x07, 0xac, 0x83, 0xb2, 0x94, 0x15, 0x19,
	0xad, 0xd8, 0xe5, 0x57, 0x16, 0x5d, 0xda, 0x0c, 0x7d, 0x6b, 0x33, 0xf4, 0xbd, 0xcd, 0xd0, 0xcf,
	0x36, 0x43, 0x5f, 0x7e, 0x67, 0xd1, 0x21, 0x0d, 0xdf, 0xbd, 0xfc, 0x13, 0x00, 0x00, 0xff, 0xff,
	0x70, 0xa2, 0xfc, 0xc5, 0x23, 0x02, 0x00, 0x00,
}
