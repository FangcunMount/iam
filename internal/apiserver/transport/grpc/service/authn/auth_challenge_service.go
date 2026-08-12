package authn

import (
	"context"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *authChallengeServiceServer) SendLoginPhoneOTP(ctx context.Context, req *authnv2.SendLoginPhoneOTPRequest) (*authnv2.MessageResponse, error) {
	if s.loginPhoneOTPSender == nil {
		return nil, status.Error(codes.Unimplemented, "challenge service not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := s.loginPhoneOTPSender.SendLoginPhoneOTP(ctx, req.GetPhone()); err != nil {
		return nil, toGRPCError(err)
	}
	return &authnv2.MessageResponse{Message: "verification code sent"}, nil
}
