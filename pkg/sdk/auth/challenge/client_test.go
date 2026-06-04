package challenge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientSendsLoginPhoneOTP(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/authn/challenges/phone-otp", r.URL.Path)

		var req SendLoginPhoneOTPRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "13800138000", req.Phone)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"message":"verification code sent"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	resp, err := client.SendLoginPhoneOTP(context.Background(), SendLoginPhoneOTPRequest{Phone: "13800138000"})

	require.NoError(t, err)
	require.Equal(t, "verification code sent", resp.Message)
}

func TestClientValidatesSendLoginPhoneOTPBeforeHTTPCall(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.SendLoginPhoneOTP(context.Background(), SendLoginPhoneOTPRequest{})

	require.Error(t, err)
	require.False(t, called)
}

func TestClientStartsWechatOpenAuthorize(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/authn/wechat-open/authorize", r.URL.Path)

		var req WechatOpenAuthorizeRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "nonce-1", req.Nonce)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"state":"state-1","nonce":"nonce-1","app_id":"wx-open-app","authorize_url":"https://open.weixin.qq.com/connect/qrconnect?state=state-1","expires_at":"2026-06-04T12:00:00Z"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	resp, err := client.StartWechatOpenAuthorize(context.Background(), WechatOpenAuthorizeRequest{Nonce: "nonce-1"})

	require.NoError(t, err)
	require.Equal(t, "state-1", resp.State)
	require.Equal(t, "nonce-1", resp.Nonce)
	require.Equal(t, "wx-open-app", resp.AppID)
	require.Equal(t, "https://open.weixin.qq.com/connect/qrconnect?state=state-1", resp.AuthorizeURL)
	require.Equal(t, expiresAt, resp.ExpiresAt)
}

func TestClientStartsWechatOpenAuthorizeWithEmptyNonce(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/authn/wechat-open/authorize", r.URL.Path)

		var req WechatOpenAuthorizeRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Empty(t, req.Nonce)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"state":"state-2","nonce":"","app_id":"wx-open-app","authorize_url":"https://open.weixin.qq.com/connect/qrconnect?state=state-2","expires_at":"2026-06-04T12:05:00Z"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	resp, err := client.StartWechatOpenAuthorize(context.Background(), WechatOpenAuthorizeRequest{})

	require.NoError(t, err)
	require.Equal(t, "state-2", resp.State)
	require.Empty(t, resp.Nonce)
}
