package types

import "github.com/cosmos/gogoproto/proto"

func init() {
	proto.RegisterType((*QueryVerificationStatusRequest)(nil), "ageverify.ageverify.QueryVerificationStatusRequest")
	proto.RegisterType((*QueryVerificationStatusResponse)(nil), "ageverify.ageverify.QueryVerificationStatusResponse")
}

type QueryVerificationStatusRequest struct {
	Address string `protobuf:"bytes,1,opt,name=address,proto3" json:"address"`
}

func (m *QueryVerificationStatusRequest) Reset()         { *m = QueryVerificationStatusRequest{} }
func (m *QueryVerificationStatusRequest) String() string  { return proto.CompactTextString(m) }
func (*QueryVerificationStatusRequest) ProtoMessage()     {}

type QueryVerificationStatusResponse struct {
	Verified   bool   `protobuf:"varint,1,opt,name=verified,proto3"    json:"verified"`
	VerifiedAt string `protobuf:"bytes,2,opt,name=verified_at,proto3"  json:"verified_at"`
}

func (m *QueryVerificationStatusResponse) Reset()         { *m = QueryVerificationStatusResponse{} }
func (m *QueryVerificationStatusResponse) String() string  { return proto.CompactTextString(m) }
func (*QueryVerificationStatusResponse) ProtoMessage()     {}
