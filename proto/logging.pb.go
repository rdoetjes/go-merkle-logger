package proto

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Messages

type LogRequest struct {
	Application string
	Level       string
	Message     string
	Fields      map[string]string
}

type LogResponse struct {
	Ok    bool
	Error string
}

type VerifyRequest struct {
	StartSequence int64
	EndSequence   int64
}

type VerifyResponse struct {
	Ok    bool
	Error string
}

// Service interface and helper to register with gRPC

type LoggerServer interface {
	Write(context.Context, *LogRequest) (*LogResponse, error)
	Verify(context.Context, *VerifyRequest) (*VerifyResponse, error)
}

// UnimplementedLoggerServer can be embedded to have forward compatible implementations.
type UnimplementedLoggerServer struct{}

func (*UnimplementedLoggerServer) Write(ctx context.Context, req *LogRequest) (*LogResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Write not implemented")
}
func (*UnimplementedLoggerServer) Verify(ctx context.Context, req *VerifyRequest) (*VerifyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Verify not implemented")
}

func RegisterLoggerServer(s *grpc.Server, srv LoggerServer) {
	if srv == nil {
		panic("nil LoggerServer")
	}
	s.RegisterService(&_Logger_serviceDesc, srv)
}

var _Logger_serviceDesc = grpc.ServiceDesc{
	ServiceName: "merkle.logging.Logger",
	HandlerType: (*LoggerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Write",
			Handler:    _Logger_Write_Handler,
		},
		{
			MethodName: "Verify",
			Handler:    _Logger_Verify_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "logging.proto",
}

func _Logger_Write_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LogRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LoggerServer).Write(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/merkle.logging.Logger/Write",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LoggerServer).Write(ctx, req.(*LogRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Logger_Verify_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(VerifyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LoggerServer).Verify(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/merkle.logging.Logger/Verify",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LoggerServer).Verify(ctx, req.(*VerifyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Convenience compile-time check
var _ = fmt.Sprintf
