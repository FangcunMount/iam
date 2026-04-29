package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/internal/apiserver/application/authn/login"
)

type loginServiceCaptureStub struct {
	called bool
	req    login.LoginRequest
}

func (s *loginServiceCaptureStub) Login(_ context.Context, req login.LoginRequest) (*login.LoginResult, error) {
	s.called = true
	s.req = req
	return &login.LoginResult{}, nil
}

func (s *loginServiceCaptureStub) Logout(context.Context, login.LogoutRequest) error {
	return nil
}

func TestAuthHandlerLoginV1PasswordAdapterKeepsLegacySelection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stub := &loginServiceCaptureStub{}
	h := NewAuthHandler(stub, nil, nil)

	w := performAuthRequest(h.Login, `{
		"method": "password",
		"credentials": {
			"username": "alice",
			"password": "secret",
			"tenant_id": 42
		}
	}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, stub.called)
	require.Equal(t, login.ScenarioSelectionLegacy, stub.req.SelectionMode)
	require.Equal(t, login.AuthTypePassword, stub.req.AuthType)
	require.NotNil(t, stub.req.Username)
	require.NotNil(t, stub.req.Password)
	require.Equal(t, "alice", *stub.req.Username)
	require.Equal(t, "secret", *stub.req.Password)
	require.Equal(t, uint64(42), stub.req.TenantID.Uint64())
}

func TestAuthHandlerLoginV1AdaptersKeepLegacySelection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		body     string
		wantType login.AuthType
		assert   func(t *testing.T, req login.LoginRequest)
	}{
		{
			name: "phone otp",
			body: `{
				"method": "phone_otp",
				"credentials": {
					"phone": "+8613800138000",
					"otp_code": "123456"
				}
			}`,
			wantType: login.AuthTypePhoneOTP,
			assert: func(t *testing.T, req login.LoginRequest) {
				require.NotNil(t, req.PhoneE164)
				require.NotNil(t, req.OTPCode)
				require.Equal(t, "+8613800138000", *req.PhoneE164)
				require.Equal(t, "123456", *req.OTPCode)
			},
		},
		{
			name: "wechat",
			body: `{
				"method": "wechat",
				"credentials": {
					"app_id": "wx-app",
					"code": "js-code"
				}
			}`,
			wantType: login.AuthTypeWechat,
			assert: func(t *testing.T, req login.LoginRequest) {
				require.NotNil(t, req.WechatAppID)
				require.NotNil(t, req.WechatJSCode)
				require.Equal(t, "wx-app", *req.WechatAppID)
				require.Equal(t, "js-code", *req.WechatJSCode)
			},
		},
		{
			name: "wecom",
			body: `{
				"method": "wecom",
				"credentials": {
					"corp_id": "corp-id",
					"auth_code": "auth-code"
				}
			}`,
			wantType: login.AuthTypeWecom,
			assert: func(t *testing.T, req login.LoginRequest) {
				require.NotNil(t, req.WecomCorpID)
				require.NotNil(t, req.WecomCode)
				require.Equal(t, "corp-id", *req.WecomCorpID)
				require.Equal(t, "auth-code", *req.WecomCode)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stub := &loginServiceCaptureStub{}
			h := NewAuthHandler(stub, nil, nil)

			w := performAuthRequest(h.Login, tc.body)

			require.Equal(t, http.StatusOK, w.Code)
			require.True(t, stub.called)
			require.Equal(t, login.ScenarioSelectionLegacy, stub.req.SelectionMode)
			require.Equal(t, tc.wantType, stub.req.AuthType)
			tc.assert(t, stub.req)
		})
	}
}

func TestAuthHandlerLoginV2AdaptersUseExplicitSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		body     string
		wantType login.AuthType
		assert   func(t *testing.T, req login.LoginRequest)
	}{
		{
			name: "password",
			body: `{
				"auth_method": "password",
				"method_payload": {
					"username": "alice",
					"password": "secret",
					"tenant_id": 77
				}
			}`,
			wantType: login.AuthTypePassword,
			assert: func(t *testing.T, req login.LoginRequest) {
				require.NotNil(t, req.Username)
				require.NotNil(t, req.Password)
				require.Equal(t, "alice", *req.Username)
				require.Equal(t, "secret", *req.Password)
				require.Equal(t, uint64(77), req.TenantID.Uint64())
			},
		},
		{
			name: "phone otp",
			body: `{
				"auth_method": "phone_otp",
				"method_payload": {
					"phone": "+8613800138000",
					"otp_code": "123456"
				}
			}`,
			wantType: login.AuthTypePhoneOTP,
			assert: func(t *testing.T, req login.LoginRequest) {
				require.NotNil(t, req.PhoneE164)
				require.NotNil(t, req.OTPCode)
				require.Equal(t, "+8613800138000", *req.PhoneE164)
				require.Equal(t, "123456", *req.OTPCode)
			},
		},
		{
			name: "wechat",
			body: `{
				"auth_method": "wechat",
				"method_payload": {
					"app_id": "wx-app",
					"code": "js-code"
				}
			}`,
			wantType: login.AuthTypeWechat,
			assert: func(t *testing.T, req login.LoginRequest) {
				require.NotNil(t, req.WechatAppID)
				require.NotNil(t, req.WechatJSCode)
				require.Equal(t, "wx-app", *req.WechatAppID)
				require.Equal(t, "js-code", *req.WechatJSCode)
			},
		},
		{
			name: "wecom",
			body: `{
				"auth_method": "wecom",
				"method_payload": {
					"corp_id": "corp-id",
					"auth_code": "auth-code"
				}
			}`,
			wantType: login.AuthTypeWecom,
			assert: func(t *testing.T, req login.LoginRequest) {
				require.NotNil(t, req.WecomCorpID)
				require.NotNil(t, req.WecomCode)
				require.Equal(t, "corp-id", *req.WecomCorpID)
				require.Equal(t, "auth-code", *req.WecomCode)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stub := &loginServiceCaptureStub{}
			h := NewAuthHandler(stub, nil, nil)

			w := performAuthRequest(h.LoginV2, tc.body)

			require.Equal(t, http.StatusOK, w.Code)
			require.True(t, stub.called)
			require.Equal(t, login.ScenarioSelectionExplicit, stub.req.SelectionMode)
			require.Equal(t, tc.wantType, stub.req.AuthType)
			tc.assert(t, stub.req)
		})
	}
}

func TestAuthHandlerLoginV2RejectsInvalidContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "unknown auth method",
			body: `{"auth_method":"jwt_token","method_payload":{"access_token":"token"}}`,
		},
		{
			name: "missing method payload",
			body: `{"auth_method":"password"}`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stub := &loginServiceCaptureStub{}
			h := NewAuthHandler(stub, nil, nil)

			w := performAuthRequest(h.LoginV2, tc.body)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.False(t, stub.called)
		})
	}
}

func performAuthRequest(handler gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler(c)
	return w
}
