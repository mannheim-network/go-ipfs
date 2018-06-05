// Code generated by protoc-gen-gogo.
// source: namesys/pb/namesys.proto
// DO NOT EDIT!

/*
Package namesys_pb is a generated protocol buffer package.

It is generated from these files:
	namesys/pb/namesys.proto

It has these top-level messages:
	IpnsEntry
*/
package namesys_pb

import proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

type IpnsEntry_ValidityType int32

const (
	// setting an EOL says "this record is valid until..."
	IpnsEntry_EOL IpnsEntry_ValidityType = 0
)

var IpnsEntry_ValidityType_name = map[int32]string{
	0: "EOL",
}
var IpnsEntry_ValidityType_value = map[string]int32{
	"EOL": 0,
}

func (x IpnsEntry_ValidityType) Enum() *IpnsEntry_ValidityType {
	p := new(IpnsEntry_ValidityType)
	*p = x
	return p
}
func (x IpnsEntry_ValidityType) String() string {
	return proto.EnumName(IpnsEntry_ValidityType_name, int32(x))
}
func (x *IpnsEntry_ValidityType) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(IpnsEntry_ValidityType_value, data, "IpnsEntry_ValidityType")
	if err != nil {
		return err
	}
	*x = IpnsEntry_ValidityType(value)
	return nil
}

type IpnsEntry struct {
	Value        []byte                  `protobuf:"bytes,1,req,name=value" json:"value,omitempty"`
	Signature    []byte                  `protobuf:"bytes,2,req,name=signature" json:"signature,omitempty"`
	ValidityType *IpnsEntry_ValidityType `protobuf:"varint,3,opt,name=validityType,enum=namesys.pb.IpnsEntry_ValidityType" json:"validityType,omitempty"`
	Validity     []byte                  `protobuf:"bytes,4,opt,name=validity" json:"validity,omitempty"`
	Sequence     *uint64                 `protobuf:"varint,5,opt,name=sequence" json:"sequence,omitempty"`
	Ttl          *uint64                 `protobuf:"varint,6,opt,name=ttl" json:"ttl,omitempty"`
	// in order for nodes to properly validate a record upon receipt, they need the public
	// key associated with it. For old RSA keys, its easiest if we just send this as part of
	// the record itself. For newer ed25519 keys, the public key can be embedded in the
	// peerID, making this field unnecessary.
	PubKey           []byte `protobuf:"bytes,7,opt,name=pubKey" json:"pubKey,omitempty"`
	XXX_unrecognized []byte `json:"-"`
}

func (m *IpnsEntry) Reset()         { *m = IpnsEntry{} }
func (m *IpnsEntry) String() string { return proto.CompactTextString(m) }
func (*IpnsEntry) ProtoMessage()    {}

func (m *IpnsEntry) GetValue() []byte {
	if m != nil {
		return m.Value
	}
	return nil
}

func (m *IpnsEntry) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *IpnsEntry) GetValidityType() IpnsEntry_ValidityType {
	if m != nil && m.ValidityType != nil {
		return *m.ValidityType
	}
	return IpnsEntry_EOL
}

func (m *IpnsEntry) GetValidity() []byte {
	if m != nil {
		return m.Validity
	}
	return nil
}

func (m *IpnsEntry) GetSequence() uint64 {
	if m != nil && m.Sequence != nil {
		return *m.Sequence
	}
	return 0
}

func (m *IpnsEntry) GetTtl() uint64 {
	if m != nil && m.Ttl != nil {
		return *m.Ttl
	}
	return 0
}

func (m *IpnsEntry) GetPubKey() []byte {
	if m != nil {
		return m.PubKey
	}
	return nil
}

func init() {
	proto.RegisterType((*IpnsEntry)(nil), "namesys.pb.IpnsEntry")
	proto.RegisterEnum("namesys.pb.IpnsEntry_ValidityType", IpnsEntry_ValidityType_name, IpnsEntry_ValidityType_value)
}
