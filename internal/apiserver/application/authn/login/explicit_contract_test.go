package login

import (
	"encoding/json"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPublicAuthTypesDerivedFromApplicationCatalog(t *testing.T) {
	require.ElementsMatch(t, []AuthType{
		AuthTypePassword,
		AuthTypePhoneOTP,
		AuthTypeWechat,
		AuthTypeWecom,
	}, PublicAuthTypes())
	require.False(t, IsPublicAuthType(string(AuthTypeJWTToken)))
}

func TestBuildExplicitLoginRequestMapsPublicV2Payloads(t *testing.T) {
	tests := []struct {
		name    string
		method  AuthType
		payload string
		assert  func(*testing.T, LoginRequest)
	}{
		{
			name:    "password",
			method:  AuthTypePassword,
			payload: `{"username":"alice","password":"secret","tenant_id":42}`,
			assert: func(t *testing.T, req LoginRequest) {
				require.Equal(t, AuthTypePassword, req.AuthType)
				require.Equal(t, SignInSelectionExplicit, req.SelectionMode)
				require.Equal(t, "alice", *req.Username)
				require.Equal(t, "secret", *req.Password)
				require.Equal(t, meta.FromUint64(42), req.TenantID)
			},
		},
		{
			name:    "phone otp",
			method:  AuthTypePhoneOTP,
			payload: `{"phone":"+8613800138000","otp_code":"123456"}`,
			assert: func(t *testing.T, req LoginRequest) {
				require.Equal(t, AuthTypePhoneOTP, req.AuthType)
				require.Equal(t, "+8613800138000", *req.PhoneE164)
				require.Equal(t, "123456", *req.OTPCode)
			},
		},
		{
			name:    "wechat",
			method:  AuthTypeWechat,
			payload: `{"app_id":"wx-app","code":"js-code"}`,
			assert: func(t *testing.T, req LoginRequest) {
				require.Equal(t, AuthTypeWechat, req.AuthType)
				require.Equal(t, "wx-app", *req.WechatAppID)
				require.Equal(t, "js-code", *req.WechatJSCode)
			},
		},
		{
			name:    "wecom",
			method:  AuthTypeWecom,
			payload: `{"corp_id":"corp","auth_code":"auth-code"}`,
			assert: func(t *testing.T, req LoginRequest) {
				require.Equal(t, AuthTypeWecom, req.AuthType)
				require.Equal(t, "corp", *req.WecomCorpID)
				require.Equal(t, "auth-code", *req.WecomCode)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req, err := BuildExplicitLoginRequest(string(tc.method), json.RawMessage(tc.payload))
			require.NoError(t, err)
			tc.assert(t, req)
		})
	}
}

func TestBuildExplicitLoginRequestRejectsNonPublicCatalogMethod(t *testing.T) {
	_, err := BuildExplicitLoginRequest(string(AuthTypeJWTToken), json.RawMessage(`{"token":"jwt"}`))
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}
