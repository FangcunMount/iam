package authn

import (
	"context"

	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	linkingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *loginIdentityServiceServer) ListLoginIdentities(ctx context.Context, req *authnv2.ListLoginIdentitiesRequest) (*authnv2.ListLoginIdentitiesResponse, error) {
	if s.linking == nil {
		return nil, status.Error(codes.Unimplemented, "login identity service not configured")
	}
	userID, _, _, err := parseAuthenticatedUserContext(req.GetActor())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	identities, err := s.linking.List(ctx, userID)
	if err != nil {
		return nil, toGRPCError(err)
	}
	out := make([]*authnv2.LoginIdentity, 0, len(identities))
	for _, identity := range identities {
		out = append(out, toProtoLoginIdentityView(identity))
	}
	return &authnv2.ListLoginIdentitiesResponse{Items: out}, nil
}

func (s *loginIdentityServiceServer) SendPhoneLinkChallenge(ctx context.Context, req *authnv2.SendPhoneLinkChallengeRequest) (*authnv2.MessageResponse, error) {
	if s.linking == nil {
		return nil, status.Error(codes.Unimplemented, "login identity service not configured")
	}
	userID, _, _, err := parseAuthenticatedUserContext(req.GetActor())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.linking.SendPhoneLinkChallenge(ctx, userID, req.GetPhone()); err != nil {
		return nil, toGRPCError(err)
	}
	return &authnv2.MessageResponse{Message: "verification code sent"}, nil
}

func (s *loginIdentityServiceServer) LinkPhone(ctx context.Context, req *authnv2.LinkPhoneRequest) (*authnv2.LinkLoginIdentityResponse, error) {
	if s.linking == nil {
		return nil, status.Error(codes.Unimplemented, "login identity service not configured")
	}
	userID, _, _, err := parseAuthenticatedUserContext(req.GetActor())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	result, err := s.linking.LinkPhone(ctx, linkingApp.LinkPhoneCommand{
		UserID:  userID,
		Phone:   req.GetPhone(),
		OTPCode: req.GetOtpCode(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoLinkResult(result), nil
}

func (s *loginIdentityServiceServer) LinkWechatMiniProgram(ctx context.Context, req *authnv2.LinkWechatMiniProgramRequest) (*authnv2.LinkLoginIdentityResponse, error) {
	if s.linking == nil {
		return nil, status.Error(codes.Unimplemented, "login identity service not configured")
	}
	userID, _, _, err := parseAuthenticatedUserContext(req.GetActor())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	result, err := s.linking.LinkWechatMini(ctx, linkingApp.LinkWechatMiniCommand{
		UserID: userID,
		AppID:  req.GetAppId(),
		Code:   req.GetCode(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoLinkResult(result), nil
}

func (s *loginIdentityServiceServer) LinkWecom(ctx context.Context, req *authnv2.LinkWecomRequest) (*authnv2.LinkLoginIdentityResponse, error) {
	if s.linking == nil {
		return nil, status.Error(codes.Unimplemented, "login identity service not configured")
	}
	userID, _, _, err := parseAuthenticatedUserContext(req.GetActor())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	result, err := s.linking.LinkWecom(ctx, linkingApp.LinkWecomCommand{
		UserID: userID,
		CorpID: req.GetCorpId(),
		Code:   req.GetCode(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoLinkResult(result), nil
}

func (s *loginIdentityServiceServer) UnlinkLoginIdentity(ctx context.Context, req *authnv2.UnlinkLoginIdentityRequest) (*authnv2.MessageResponse, error) {
	if s.linking == nil {
		return nil, status.Error(codes.Unimplemented, "login identity service not configured")
	}
	userID, currentID, authenticatedAt, err := parseAuthenticatedUserContext(req.GetActor())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	loginIdentityID, err := parseRequiredMetaID(req.GetLoginIdentityId(), "login_identity_id")
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.linking.Unlink(ctx, linkingApp.UnlinkCommand{
		UserID:                 userID,
		LoginIdentityID:        loginIdentityID,
		CurrentLoginIdentityID: currentID,
		AuthenticatedAt:        authenticatedAt,
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &authnv2.MessageResponse{Message: "login identity unlinked"}, nil
}
