package authn

import (
	"context"
	"encoding/json"
	"strings"

	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	loginApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// Login implements the v2 explicit auth_method + method_payload login contract.
func (s *authServiceServer) Login(ctx context.Context, req *authnv2.LoginRequest) (*authnv2.LoginResponse, error) {
	if s.loginSvc == nil {
		return nil, status.Error(codes.Unimplemented, "login service not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	method := strings.TrimSpace(req.GetAuthMethod())
	if method == "" {
		return nil, status.Error(codes.InvalidArgument, "auth_method is required")
	}
	if req.GetMethodPayload() == nil {
		return nil, status.Error(codes.InvalidArgument, "method_payload is required")
	}

	payload, err := protojson.Marshal(req.GetMethodPayload())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid method_payload: %v", err)
	}
	loginReq, err := loginApp.BuildExplicitLoginRequest(method, json.RawMessage(payload))
	if err != nil {
		return nil, toGRPCError(err)
	}
	result, err := s.loginSvc.Login(ctx, loginReq)
	if err != nil {
		return nil, toGRPCError(err)
	}
	if result == nil {
		return &authnv2.LoginResponse{}, nil
	}
	return &authnv2.LoginResponse{TokenPair: toProtoTokenPair(result.TokenPair)}, nil
}
