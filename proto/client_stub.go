package proto

import (
	"context"

	"google.golang.org/grpc"
)

type loggerClient struct {
	cc *grpc.ClientConn
}

func NewLoggerClient(cc *grpc.ClientConn) LoggerClient {
	return &loggerClient{cc: cc}
}

type LoggerClient interface {
	Write(ctx context.Context, in *LogRequest, opts ...grpc.CallOption) (*LogResponse, error)
	Verify(ctx context.Context, in *VerifyRequest, opts ...grpc.CallOption) (*VerifyResponse, error)
}

func (c *loggerClient) Write(ctx context.Context, in *LogRequest, opts ...grpc.CallOption) (*LogResponse, error) {
	out := new(LogResponse)
	// perform actual gRPC invoke
	if err := c.cc.Invoke(ctx, "/merkle.logging.Logger/Write", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *loggerClient) Verify(ctx context.Context, in *VerifyRequest, opts ...grpc.CallOption) (*VerifyResponse, error) {
	out := new(VerifyResponse)
	if err := c.cc.Invoke(ctx, "/merkle.logging.Logger/Verify", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}
