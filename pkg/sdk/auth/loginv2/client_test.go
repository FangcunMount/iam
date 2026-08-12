package loginv2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	sdkerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
)

func TestClientLoginPostsExplicitV2Contract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/authn/login", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))

		var req struct {
			AuthMethod    AuthMethod      `json:"auth_method"`
			MethodPayload PasswordPayload `json:"method_payload"`
			DeviceID      string          `json:"device_id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, AuthMethodPassword, req.AuthMethod)
		require.Equal(t, "device-1", req.DeviceID)
		require.Equal(t, "alice", req.MethodPayload.Username)
		require.Equal(t, "secret", req.MethodPayload.Password)
		require.Equal(t, uint64(7), req.MethodPayload.TenantID)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "success",
			"data": {
				"access_token": "access-token",
				"token_type": "Bearer",
				"expires_in": 3600,
				"refresh_token": "refresh-token"
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	tokenPair, err := client.Login(context.Background(), LoginRequest{
		AuthMethod: AuthMethodPassword,
		MethodPayload: PasswordPayload{
			Username: "alice",
			Password: "secret",
			TenantID: 7,
		},
		DeviceID: "device-1",
	})

	require.NoError(t, err)
	require.Equal(t, "access-token", tokenPair.AccessToken)
	require.Equal(t, "Bearer", tokenPair.TokenType)
	require.Equal(t, int64(3600), tokenPair.ExpiresIn)
	require.Equal(t, "refresh-token", tokenPair.RefreshToken)
}

func TestClientLoginDoesNotDoubleAppendAPIV2(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/authn/login", r.URL.Path)
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"token","token_type":"Bearer","expires_in":1}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL + "/api/v2")
	require.NoError(t, err)

	_, err = client.Login(context.Background(), LoginRequest{
		AuthMethod:    AuthMethodWecom,
		MethodPayload: WecomPayload{CorpID: "corp", AuthCode: "code"},
	})
	require.NoError(t, err)
}

func TestClientLoginPostsWechatScanPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/authn/login", r.URL.Path)

		var req struct {
			AuthMethod    AuthMethod        `json:"auth_method"`
			MethodPayload WechatScanPayload `json:"method_payload"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, AuthMethodWechatScan, req.AuthMethod)
		require.Equal(t, "wx-open-app", req.MethodPayload.AppID)
		require.Equal(t, "oauth-code", req.MethodPayload.Code)
		require.Equal(t, "oauth-state", req.MethodPayload.State)

		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"access_token":"token","token_type":"Bearer","expires_in":1}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.Login(context.Background(), LoginRequest{
		AuthMethod: AuthMethodWechatScan,
		MethodPayload: WechatScanPayload{
			AppID: "wx-open-app",
			Code:  "oauth-code",
			State: "oauth-state",
		},
	})
	require.NoError(t, err)
}

func TestClientLoginReturnsIAMErrorFromRESTEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":100004,"message":"invalid token","reference":"https://docs.example.com/auth"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.Login(context.Background(), LoginRequest{
		AuthMethod:    AuthMethodPhoneOTP,
		MethodPayload: PhoneOTPPayload{Phone: "+8613800138000", OTPCode: "123456"},
	})

	require.Error(t, err)
	require.True(t, sdkerrors.IsUnauthorized(err))
	require.Equal(t, "invalid token", sdkerrors.Message(err))
}

func TestClientLoginValidatesRequestBeforeHTTPCall(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.Login(context.Background(), LoginRequest{
		AuthMethod:    AuthMethod("jwt_token"),
		MethodPayload: map[string]string{"access_token": "token"},
	})

	require.Error(t, err)
	require.False(t, called)
}

func TestLoginRequestValidateAcceptsWechatScan(t *testing.T) {
	req := LoginRequest{
		AuthMethod: AuthMethodWechatScan,
		MethodPayload: WechatScanPayload{
			AppID: "wx-open-app",
			Code:  "oauth-code",
			State: "oauth-state",
		},
	}

	require.NoError(t, req.Validate())
}
