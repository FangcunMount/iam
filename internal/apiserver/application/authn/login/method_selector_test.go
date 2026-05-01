package login

import (
	"context"
	"testing"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/stretchr/testify/require"

	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

func TestLegacyMethodSelectorPreservesOverrideOrder(t *testing.T) {
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
		want SignInKind
	}{
		{
			name: "password",
			req: LoginRequest{
				Username: &username,
				Password: &password,
			},
			want: SignInKind(credDomain.CredPassword),
		},
		{
			name: "phone overrides password",
			req: LoginRequest{
				Username:  &username,
				Password:  &password,
				PhoneE164: &phone,
				OTPCode:   &otp,
			},
			want: SignInKind(credDomain.CredPhoneOTP),
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
			want: SignInKind(credDomain.CredOAuthWxMinip),
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
			want: SignInKind(credDomain.CredOAuthWecom),
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
			want: SignInKind(AuthTypeJWTToken),
		},
	}

	selector := newDefaultMethodSelector(newDefaultSignInAdapterCatalog(signInAdapterDeps{}))
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

func TestExplicitMethodSelectorRequiresMethodFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  LoginRequest
	}{
		{
			name: "password username",
			req: LoginRequest{
				SelectionMode: SignInSelectionExplicit,
				AuthType:      AuthTypePassword,
			},
		},
		{
			name: "phone",
			req: LoginRequest{
				SelectionMode: SignInSelectionExplicit,
				AuthType:      AuthTypePhoneOTP,
			},
		},
		{
			name: "wechat",
			req: LoginRequest{
				SelectionMode: SignInSelectionExplicit,
				AuthType:      AuthTypeWechat,
			},
		},
		{
			name: "wecom",
			req: LoginRequest{
				SelectionMode: SignInSelectionExplicit,
				AuthType:      AuthTypeWecom,
			},
		},
		{
			name: "bearer",
			req: LoginRequest{
				SelectionMode: SignInSelectionExplicit,
				AuthType:      AuthTypeJWTToken,
			},
		},
	}

	selector := newDefaultMethodSelector(newDefaultSignInAdapterCatalog(signInAdapterDeps{}))
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
