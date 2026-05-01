package authn

import (
	"context"

	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *jwksServiceServer) GetJWKS(ctx context.Context, req *authnv2.GetJWKSRequest) (*authnv2.GetJWKSResponse, error) {
	if s.keyPublish == nil {
		return nil, status.Error(codes.Unimplemented, "jwks service not configured")
	}
	_ = req // reserved for future cache validation
	result, err := s.keyPublish.BuildJWKS(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authnv2.GetJWKSResponse{
		Jwks:         result.JWKS,
		Etag:         result.ETag,
		LastModified: timestamppb.New(result.LastModified),
	}, nil
}
