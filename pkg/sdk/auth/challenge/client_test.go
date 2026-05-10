package challenge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
