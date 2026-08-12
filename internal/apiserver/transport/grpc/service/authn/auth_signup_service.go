package authn

import (
	"context"
	"fmt"
	"strings"

	authnv2 "github.com/FangcunMount/iam/v3/api/grpc/iam/authn/v2"
	signupApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signup"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *authSignupServiceServer) SignUpWithWechatMiniProgram(ctx context.Context, req *authnv2.SignUpWithWechatMiniProgramRequest) (*authnv2.SignupResult, error) {
	if s.signupService == nil {
		return nil, status.Error(codes.Unimplemented, "signup service not configured")
	}
	signupReq, err := wechatMiniProgramSignupRequestFromGRPC(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	result, err := s.signupService.SignUp(ctx, signupReq)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return toProtoSignupResult(result), nil
}

func wechatMiniProgramSignupRequestFromGRPC(req *authnv2.SignUpWithWechatMiniProgramRequest) (signupApp.SignupRequest, error) {
	if req == nil {
		return signupApp.SignupRequest{}, fmt.Errorf("request is required")
	}
	var phone meta.Phone
	if raw := strings.TrimSpace(req.GetPhone()); raw != "" {
		parsed, err := meta.NewPhone(raw)
		if err != nil {
			return signupApp.SignupRequest{}, err
		}
		phone = parsed
	}
	var email meta.Email
	if raw := strings.TrimSpace(req.GetEmail()); raw != "" {
		parsed, err := meta.NewEmail(raw)
		if err != nil {
			return signupApp.SignupRequest{}, err
		}
		email = parsed
	}
	appID := strings.TrimSpace(req.GetAppId())
	jsCode := strings.TrimSpace(req.GetJsCode())
	if appID == "" || jsCode == "" {
		return signupApp.SignupRequest{}, fmt.Errorf("app_id and js_code are required")
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
	return signupApp.SignupRequest{
		User: signupApp.SignupUserInput{
			Name:  strings.TrimSpace(req.GetName()),
			Phone: phone,
			Email: email,
		},
		LoginIdentity: signupApp.WechatMiniLoginIdentityInput{
			AppID:   &appID,
			JsCode:  &jsCode,
			Profile: profile,
			Meta:    cloneAttributes(req.GetMeta()),
		},
	}, nil
}
