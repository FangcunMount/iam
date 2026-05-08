package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
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

func (s *loginServiceCaptureStub) Reauthenticate(context.Context, string) (*login.AuthResult, error) {
	return nil, nil
}

type tokenServiceCaptureStub struct {
	refreshCalled bool
	verifyCalled  bool
	revokeCalled  bool
	refreshErr    error
	verifyErr     error
	revokeErr     error
}

func (s *tokenServiceCaptureStub) IssueServiceToken(context.Context, token.IssueServiceTokenRequest) (*token.TokenIssueResult, error) {
	return nil, nil
}

func (s *tokenServiceCaptureStub) RefreshToken(context.Context, string) (*token.TokenRefreshResult, error) {
	s.refreshCalled = true
	return nil, s.refreshErr
}

func (s *tokenServiceCaptureStub) RevokeAccessToken(context.Context, string) error {
	s.revokeCalled = true
	return s.revokeErr
}

func (s *tokenServiceCaptureStub) RevokeRefreshToken(context.Context, string) error {
	return nil
}

func (s *tokenServiceCaptureStub) VerifyToken(context.Context, token.VerifyTokenRequest) (*token.TokenVerifyResult, error) {
	s.verifyCalled = true
	if s.verifyErr != nil {
		return nil, s.verifyErr
	}
	return &token.TokenVerifyResult{Valid: false}, nil
}

func TestAuthHandlerLoginV2AdaptersUseExplicitSelection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		wantMethod login.AuthMethod
		assert     func(t *testing.T, req login.LoginRequest)
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
			wantMethod: login.AuthMethodPassword,
			assert: func(t *testing.T, req login.LoginRequest) {
				payload, ok := req.Payload.(login.PasswordPayload)
				require.True(t, ok)
				require.Equal(t, "alice", payload.Username)
				require.Equal(t, "secret", payload.Password)
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
			wantMethod: login.AuthMethodPhoneOTP,
			assert: func(t *testing.T, req login.LoginRequest) {
				payload, ok := req.Payload.(login.PhoneOTPPayload)
				require.True(t, ok)
				require.Equal(t, "+8613800138000", payload.PhoneE164)
				require.Equal(t, "123456", payload.OTP)
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
			wantMethod: login.AuthMethodWechat,
			assert: func(t *testing.T, req login.LoginRequest) {
				payload, ok := req.Payload.(login.WechatMiniPayload)
				require.True(t, ok)
				require.Equal(t, "wx-app", payload.AppID)
				require.Equal(t, "js-code", payload.JSCode)
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
			wantMethod: login.AuthMethodWecom,
			assert: func(t *testing.T, req login.LoginRequest) {
				payload, ok := req.Payload.(login.WecomPayload)
				require.True(t, ok)
				require.Equal(t, "corp-id", payload.CorpID)
				require.Equal(t, "auth-code", payload.Code)
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
			require.Equal(t, tc.wantMethod, stub.req.AuthMethod)
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

func TestAuthHandlerTokenEndpointsRejectInvalidRequestsBeforeApplicationCall(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		call   func(*AuthHandler) gin.HandlerFunc
		body   string
		called func(*tokenServiceCaptureStub) bool
	}{
		{
			name:   "refresh token missing",
			call:   func(h *AuthHandler) gin.HandlerFunc { return h.RefreshToken },
			body:   `{}`,
			called: func(s *tokenServiceCaptureStub) bool { return s.refreshCalled },
		},
		{
			name:   "verify token missing",
			call:   func(h *AuthHandler) gin.HandlerFunc { return h.VerifyToken },
			body:   `{}`,
			called: func(s *tokenServiceCaptureStub) bool { return s.verifyCalled },
		},
		{
			name:   "revoke token missing",
			call:   func(h *AuthHandler) gin.HandlerFunc { return h.RevokeToken },
			body:   `{}`,
			called: func(s *tokenServiceCaptureStub) bool { return s.revokeCalled },
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tokenSvc := &tokenServiceCaptureStub{}
			h := NewAuthHandler(nil, tokenSvc, nil)

			w := performAuthRequest(tc.call(h), tc.body)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.False(t, tc.called(tokenSvc))
		})
	}
}

func TestAuthHandlerTokenEndpointsPropagateApplicationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		call func(*AuthHandler) gin.HandlerFunc
		body string
		svc  *tokenServiceCaptureStub
	}{
		{
			name: "refresh token invalid",
			call: func(h *AuthHandler) gin.HandlerFunc { return h.RefreshToken },
			body: `{"refresh_token":"refresh-token"}`,
			svc:  &tokenServiceCaptureStub{refreshErr: perrors.WithCode(code.ErrTokenInvalid, "invalid refresh")},
		},
		{
			name: "verify token invalid",
			call: func(h *AuthHandler) gin.HandlerFunc { return h.VerifyToken },
			body: `{"access_token":"access-token"}`,
			svc:  &tokenServiceCaptureStub{verifyErr: perrors.WithCode(code.ErrTokenInvalid, "invalid access")},
		},
		{
			name: "revoke token invalid",
			call: func(h *AuthHandler) gin.HandlerFunc { return h.RevokeToken },
			body: `{"access_token":"access-token"}`,
			svc:  &tokenServiceCaptureStub{revokeErr: perrors.WithCode(code.ErrTokenInvalid, "invalid access")},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := NewAuthHandler(nil, tc.svc, nil)

			w := performAuthRequest(tc.call(h), tc.body)

			require.Equal(t, http.StatusUnauthorized, w.Code)
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
