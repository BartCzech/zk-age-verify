package types

import (
	"context"

	grpc "google.golang.org/grpc"
)

// MsgServer handles MsgSubmitAgeProof transactions.
type MsgServer interface {
	SubmitAgeProof(context.Context, *MsgSubmitAgeProof) (*MsgSubmitAgeProofResponse, error)
}

// QueryServer handles VerificationStatus queries.
type QueryServer interface {
	VerificationStatus(context.Context, *QueryVerificationStatusRequest) (*QueryVerificationStatusResponse, error)
}

// RegisterMsgServer registers the MsgServer with gRPC.
func RegisterMsgServer(s grpc.ServiceRegistrar, srv MsgServer) {
	s.RegisterService(&_Msg_serviceDesc, srv)
}

// RegisterQueryServer registers the QueryServer with gRPC.
func RegisterQueryServer(s grpc.ServiceRegistrar, srv QueryServer) {
	s.RegisterService(&_Query_serviceDesc, srv)
}

func _Msg_SubmitAgeProof_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgSubmitAgeProof)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).SubmitAgeProof(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/ageverify.ageverify.Msg/SubmitAgeProof"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).SubmitAgeProof(ctx, req.(*MsgSubmitAgeProof))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_VerificationStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryVerificationStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).VerificationStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/ageverify.ageverify.Query/VerificationStatus"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).VerificationStatus(ctx, req.(*QueryVerificationStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Msg_serviceDesc = grpc.ServiceDesc{
	ServiceName: "ageverify.ageverify.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "SubmitAgeProof", Handler: _Msg_SubmitAgeProof_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "ageverify/ageverify/v1/tx.proto",
}

var _Query_serviceDesc = grpc.ServiceDesc{
	ServiceName: "ageverify.ageverify.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "VerificationStatus", Handler: _Query_VerificationStatus_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "ageverify/ageverify/v1/query.proto",
}
