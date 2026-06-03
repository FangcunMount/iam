package session

import (
	"encoding/json"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPublicAuthMethodsDerivedFromApplicationCatalog(t *testing.T) {
	require.ElementsMatch(t, []AuthMethod{
		AuthMethodPassword,
		AuthMethodPhoneOTP,
		AuthMethodWechatMini,
		AuthMethodWechatScan,
		AuthMethodWecom,
	}, PublicAuthMethods())
	require.False(t, IsPublicAuthMethod("unsupported"))
}

func TestBuildExplicitLoginRequestMapsPublicV2Payloads(t *testing.T) {
	tests := []struct {
		name    string
		method  AuthMethod
		payload string
		assert  func(*testing.T, LoginRequest)
	}{
		{
			name:    "password",
			method:  AuthMethodPassword,
			payload: `{"username":"alice","password":"secret","tenant_id":42}`,
			assert: func(t *testing.T, req LoginRequest) {
				require.Equal(t, AuthMethodPassword, req.AuthMethod)
				require.Equal(t, meta.FromUint64(42), req.TenantID)
				payload, ok := req.Payload.(PasswordPayload)
				require.True(t, ok)
				require.Equal(t, "alice", payload.Username)
				require.Equal(t, "secret", payload.Password)
			},
		},
		{
			name:    "phone otp",
			method:  AuthMethodPhoneOTP,
			payload: `{"phone":"+8613800138000","otp_code":"123456"}`,
			assert: func(t *testing.T, req LoginRequest) {
				require.Equal(t, AuthMethodPhoneOTP, req.AuthMethod)
				payload, ok := req.Payload.(PhoneOTPPayload)
				require.True(t, ok)
				require.Equal(t, "+8613800138000", payload.PhoneE164)
				require.Equal(t, "123456", payload.OTP)
			},
		},
		{
			name:    "wechat",
			method:  AuthMethodWechatMini,
			payload: `{"app_id":"wx-app","code":"js-code"}`,
			assert: func(t *testing.T, req LoginRequest) {
				require.Equal(t, AuthMethodWechatMini, req.AuthMethod)
				payload, ok := req.Payload.(WechatMiniPayload)
				require.True(t, ok)
				require.Equal(t, "wx-app", payload.AppID)
				require.Equal(t, "js-code", payload.JSCode)
			},
		},
		{
			name:    "wechat_scan",
			method:  AuthMethodWechatScan,
			payload: `{"app_id":"wx-app","code":"scan-code"}`,
			assert: func(t *testing.T, req LoginRequest) {
				require.Equal(t, AuthMethodWechatScan, req.AuthMethod)
				payload, ok := req.Payload.(WechatScanPayload)
				require.True(t, ok)
				require.Equal(t, "wx-app", payload.AppID)
				require.Equal(t, "scan-code", payload.Code)
			},
		},
		{
			name:    "wecom",
			method:  AuthMethodWecom,
			payload: `{"corp_id":"corp","auth_code":"auth-code"}`,
			assert: func(t *testing.T, req LoginRequest) {
				require.Equal(t, AuthMethodWecom, req.AuthMethod)
				payload, ok := req.Payload.(WecomPayload)
				require.True(t, ok)
				require.Equal(t, "corp", payload.CorpID)
				require.Equal(t, "auth-code", payload.Code)
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
	_, err := BuildExplicitLoginRequest("unsupported", json.RawMessage(`{"value":"x"}`))
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrUnsupportedAuthMethod))
}
