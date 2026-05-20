package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ageverify/x/ageverify/types"
)

func (k Keeper) VerificationStatus(
	goCtx context.Context,
	req *types.QueryVerificationStatusRequest,
) (*types.QueryVerificationStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	verified, verifiedAt, found := k.GetVerified(goCtx, req.Address)
	if !found {
		return &types.QueryVerificationStatusResponse{
			Verified:   false,
			VerifiedAt: "",
		}, nil
	}
	return &types.QueryVerificationStatusResponse{
		Verified:   verified,
		VerifiedAt: verifiedAt,
	}, nil
}
