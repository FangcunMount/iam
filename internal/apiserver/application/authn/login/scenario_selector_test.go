package login

import (
	"context"
	"testing"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/pkg/code"
)

func TestLegacyScenarioSelectorPreservesOverrideOrder(t *testing.T) {
	t.Parallel()

	username := "alice"
	password := "secret"
	phone := "+8613800138000"
	otp := "123456"
	wechatAppID := "wx-app"
	wechatCode := "wx-code"
	wecomCorpID := "corp-id"
	wecomCode := "wecom-code"
	jwtToken := "jwt-token"

	tests := []struct {
		name string
		req  LoginRequest
		want MethodKind
	}{
		{
			name: "password",
			req: LoginRequest{
				Username: &username,
				Password: &password,
			},
			want: MethodPassword,
		},
		{
			name: "phone overrides password",
			req: LoginRequest{
				Username:  &username,
				Password:  &password,
				PhoneE164: &phone,
				OTPCode:   &otp,
			},
			want: MethodPhoneOTP,
		},
		{
			name: "wechat overrides phone",
			req: LoginRequest{
				Username:     &username,
				Password:     &password,
				PhoneE164:    &phone,
				OTPCode:      &otp,
				WechatAppID:  &wechatAppID,
				WechatJSCode: &wechatCode,
			},
			want: MethodWechatMini,
		},
		{
			name: "wecom overrides wechat",
			req: LoginRequest{
				Username:     &username,
				Password:     &password,
				PhoneE164:    &phone,
				OTPCode:      &otp,
				WechatAppID:  &wechatAppID,
				WechatJSCode: &wechatCode,
				WecomCorpID:  &wecomCorpID,
				WecomCode:    &wecomCode,
			},
			want: MethodWecom,
		},
		{
			name: "bearer overrides all legacy credentials",
			req: LoginRequest{
				Username:     &username,
				Password:     &password,
				PhoneE164:    &phone,
				OTPCode:      &otp,
				WechatAppID:  &wechatAppID,
				WechatJSCode: &wechatCode,
				WecomCorpID:  &wecomCorpID,
				WecomCode:    &wecomCode,
				JWTToken:     &jwtToken,
			},
			want: MethodBearerToken,
		},
	}

	selector := newDefaultScenarioSelector()
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			selected, err := selector.Select(context.Background(), tc.req)

			require.NoError(t, err)
			require.Equal(t, tc.want, selected.Method)
		})
	}
}

func TestExplicitScenarioSelectorRequiresMethodFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  LoginRequest
	}{
		{
			name: "password username",
			req: LoginRequest{
				SelectionMode: ScenarioSelectionExplicit,
				AuthType:      AuthTypePassword,
			},
		},
		{
			name: "phone",
			req: LoginRequest{
				SelectionMode: ScenarioSelectionExplicit,
				AuthType:      AuthTypePhoneOTP,
			},
		},
		{
			name: "wechat",
			req: LoginRequest{
				SelectionMode: ScenarioSelectionExplicit,
				AuthType:      AuthTypeWechat,
			},
		},
		{
			name: "wecom",
			req: LoginRequest{
				SelectionMode: ScenarioSelectionExplicit,
				AuthType:      AuthTypeWecom,
			},
		},
		{
			name: "bearer",
			req: LoginRequest{
				SelectionMode: ScenarioSelectionExplicit,
				AuthType:      AuthTypeJWTToken,
			},
		},
	}

	selector := newDefaultScenarioSelector()
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := selector.Select(context.Background(), tc.req)

			require.Error(t, err)
			require.Equal(t, code.ErrInvalidArgument, errors.ParseCoder(err).Code())
		})
	}
}
