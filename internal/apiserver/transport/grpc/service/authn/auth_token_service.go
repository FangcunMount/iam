package authn

import (
	"context"
	"strings"
	"time"

	authnv1 "github.com/FangcunMount/iam/api/grpc/iam/authn/v1"
	tokenApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *authServiceServer) VerifyToken(ctx context.Context, req *authnv1.VerifyTokenRequest) (*authnv1.VerifyTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetAccessToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}

	result, err := s.tokenSvc.VerifyToken(ctx, tokenApp.VerifyTokenRequest{
		AccessToken:      req.GetAccessToken(),
		ExpectedIssuer:   strings.TrimSpace(req.GetExpectedIssuer()),
		ExpectedAudience: cloneAudience(req.GetExpectedAudience()),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &authnv1.VerifyTokenResponse{}
	if result != nil {
		resp.Valid = result.Valid
	}
	if resp.Valid {
		resp.Status = authnv1.TokenStatus_TOKEN_STATUS_VALID
		resp.Claims = toProtoTokenClaims(result.Claims)
		if req.GetIncludeMetadata() {
			resp.Metadata = buildTokenMetadata(result.Claims)
		}
	} else {
		resp.Status = authnv1.TokenStatus_TOKEN_STATUS_REVOKED
		resp.FailureReason = "token invalid or expired"
	}
	return resp, nil
}

func (s *authServiceServer) RefreshToken(ctx context.Context, req *authnv1.RefreshTokenRequest) (*authnv1.RefreshTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetRefreshToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	result, err := s.tokenSvc.RefreshToken(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authnv1.RefreshTokenResponse{
		TokenPair: toProtoTokenPair(result.TokenPair),
	}, nil
}

func (s *authServiceServer) RevokeToken(ctx context.Context, req *authnv1.RevokeTokenRequest) (*authnv1.RevokeTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetAccessToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "access_token is required")
	}
	if err := s.tokenSvc.RevokeAccessToken(ctx, req.GetAccessToken()); err != nil {
		return nil, toGRPCError(err)
	}
	return &authnv1.RevokeTokenResponse{}, nil
}

func (s *authServiceServer) RevokeRefreshToken(ctx context.Context, req *authnv1.RevokeRefreshTokenRequest) (*authnv1.RevokeRefreshTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetRefreshToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}
	if err := s.tokenSvc.RevokeRefreshToken(ctx, req.GetRefreshToken()); err != nil {
		return nil, toGRPCError(err)
	}
	return &authnv1.RevokeRefreshTokenResponse{}, nil
}

func (s *authServiceServer) IssueServiceToken(ctx context.Context, req *authnv1.IssueServiceTokenRequest) (*authnv1.IssueServiceTokenResponse, error) {
	if s.tokenSvc == nil {
		return nil, status.Error(codes.Unimplemented, "token service not configured")
	}
	if req == nil || strings.TrimSpace(req.GetSubject()) == "" {
		return nil, status.Error(codes.InvalidArgument, "subject is required")
	}

	var ttl time.Duration
	if req.GetTtl() != nil {
		ttl = req.GetTtl().AsDuration()
		if ttl < 0 {
			return nil, status.Error(codes.InvalidArgument, "ttl must be non-negative")
		}
	}

	var attrs map[string]string
	if req.GetAttributes() != nil {
		attrs = structToStringMap(req.GetAttributes().AsMap())
	}

	result, err := s.tokenSvc.IssueServiceToken(ctx, tokenApp.IssueServiceTokenRequest{
		Subject:    strings.TrimSpace(req.GetSubject()),
		Audience:   cloneAudience(req.GetAudience()),
		TTL:        ttl,
		Attributes: attrs,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &authnv1.IssueServiceTokenResponse{
		TokenPair: toProtoTokenPair(result.TokenPair),
	}, nil
}
