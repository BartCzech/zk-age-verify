package types

import (
	"context"

	"google.golang.org/grpc"
)

// QueryClient is the client API for the Query service.
type QueryClient interface {
	VerificationStatus(ctx context.Context, in *QueryVerificationStatusRequest, opts ...grpc.CallOption) (*QueryVerificationStatusResponse, error)
}

type queryClient struct {
	cc grpc.ClientConnInterface
}

// NewQueryClient creates a new query client from a gRPC connection.
func NewQueryClient(cc grpc.ClientConnInterface) QueryClient {
	return &queryClient{cc}
}

func (c *queryClient) VerificationStatus(ctx context.Context, in *QueryVerificationStatusRequest, opts ...grpc.CallOption) (*QueryVerificationStatusResponse, error) {
	out := new(QueryVerificationStatusResponse)
	err := c.cc.Invoke(ctx, "/ageverify.ageverify.Query/VerificationStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}
