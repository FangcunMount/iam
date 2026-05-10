package authn

import (
	"context"
	"fmt"
	"strings"

	authnv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authn/v2"
	onboardingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/onboarding"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *authSignupServiceServer) SignUpWithWechatMiniProgram(ctx context.Context, req *authnv2.SignUpWithWechatMiniProgramRequest) (*authnv2.SignupResult, error) {
	if s.onboarder == nil {
		return nil, status.Error(codes.Unimplemented, "signup service not configured")
	}
	onboardingReq, err := wechatMiniProgramSignupRequestFromGRPC(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	result, err := s.onboarder.Onboard(ctx, onboardingReq)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoSignupResult(result), nil
}

func wechatMiniProgramSignupRequestFromGRPC(req *authnv2.SignUpWithWechatMiniProgramRequest) (onboardingApp.OnboardingRequest, error) {
	if req == nil {
		return onboardingApp.OnboardingRequest{}, fmt.Errorf("request is required")
	}
	var phone meta.Phone
	if raw := strings.TrimSpace(req.GetPhone()); raw != "" {
		parsed, err := meta.NewPhone(raw)
		if err != nil {
			return onboardingApp.OnboardingRequest{}, err
		}
		phone = parsed
	}
	var email meta.Email
	if raw := strings.TrimSpace(req.GetEmail()); raw != "" {
		parsed, err := meta.NewEmail(raw)
		if err != nil {
			return onboardingApp.OnboardingRequest{}, err
		}
		email = parsed
	}
	appID := strings.TrimSpace(req.GetAppId())
	jsCode := strings.TrimSpace(req.GetJsCode())
	if appID == "" || jsCode == "" {
		return onboardingApp.OnboardingRequest{}, fmt.Errorf("app_id and js_code are required")
	}
	profile := map[string]string{}
	if nickname := strings.TrimSpace(req.GetNickname()); nickname != "" {
		profile["nickname"] = nickname
	}
	if avatar := strings.TrimSpace(req.GetAvatar()); avatar != "" {
		profile["avatar"] = avatar
	}
	if len(profile) == 0 {
		profile = nil
	}
	return onboardingApp.OnboardingRequest{
		User: onboardingApp.OnboardingUserInput{
			Name:  strings.TrimSpace(req.GetName()),
			Phone: phone,
			Email: email,
		},
		LoginIdentity: onboardingApp.WechatMiniLoginIdentityInput{
			AppID:   &appID,
			JsCode:  &jsCode,
			Profile: profile,
			Meta:    cloneAttributes(req.GetMeta()),
		},
	}, nil
}
