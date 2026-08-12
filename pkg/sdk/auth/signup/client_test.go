package signup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"github.com/stretchr/testify/require"
)

func TestClientSignsUpWithWechatMiniProgram(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/authn/signups/wechat-miniprogram", r.URL.Path)

		var req WechatMiniProgramRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "Alice", req.Name)
		require.Equal(t, "+8613800138000", req.Phone)
		require.Equal(t, "alice@example.com", req.Email)
		require.Equal(t, "wx-app", req.AppID)
		require.Equal(t, "js-code", req.JsCode)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "success",
			"data": {
				"userId": 10,
				"userName": "Alice",
				"phone": "+8613800138000",
				"email": "alice@example.com",
				"loginIdentityId": 20,
				"credential": null,
				"isNewUser": true,
				"isNewIdentity": true
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	resp, err := client.SignUpWithWechatMiniProgram(context.Background(), WechatMiniProgramRequest{
		Name:   "Alice",
		Phone:  "+8613800138000",
		Email:  "alice@example.com",
		AppID:  "wx-app",
		JsCode: "js-code",
	})

	require.NoError(t, err)
	require.Equal(t, uint64(10), resp.UserID)
	require.Equal(t, uint64(20), resp.LoginIdentityID)
	require.Nil(t, resp.Credential)
}

func TestClientEnsuresMockConsumer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/internal/authn/mock-consumers/ensure", r.URL.Path)
		require.Equal(t, "seed-secret", r.Header.Get(SeedMockSecretHeader))

		var req EnsureMockConsumerRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "mock@example.com", req.Email)
		require.Equal(t, "Mock User", req.Profile["nickname"])
		require.Equal(t, "daily_simulation", req.Meta["source"])
		require.Equal(t, "2026-08-02", req.Meta["run_date"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "success",
			"data": {
				"user_id": "10",
				"login_identity_id": "20",
				"login_id": "mock@example.com",
				"is_new_user": true,
				"is_new_identity": true
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/api/v2", WithSeedMockSecret("seed-secret"))
	require.NoError(t, err)

	resp, err := client.EnsureMockConsumer(context.Background(), EnsureMockConsumerRequest{
		Name:     "Mock User",
		Phone:    "13800138000",
		Email:    "mock@example.com",
		Password: "secret",
		Profile:  map[string]string{"nickname": "Mock User"},
		Meta:     map[string]string{"source": "daily_simulation", "run_date": "2026-08-02"},
	})

	require.NoError(t, err)
	require.Equal(t, "10", resp.UserID)
	require.Equal(t, "20", resp.LoginIdentityID)
	require.Equal(t, "mock@example.com", resp.LoginID)
}

func TestClientReturnsIAMErrorFromEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":100004,"message":"seed mock secret invalid"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.EnsureMockConsumer(context.Background(), EnsureMockConsumerRequest{
		Name:     "Mock User",
		Phone:    "13800138000",
		Email:    "mock@example.com",
		Password: "secret",
	})

	require.Error(t, err)
	require.True(t, sdkerrors.IsUnauthorized(err))
	require.Equal(t, "seed mock secret invalid", sdkerrors.Message(err))
}

func TestClientValidatesRequestBeforeHTTPCall(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.SignUpWithWechatMiniProgram(context.Background(), WechatMiniProgramRequest{})

	require.Error(t, err)
	require.False(t, called)
}

func TestClientValidatesWechatMiniProgramOptionalContactsBeforeHTTPCall(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	tests := []struct {
		name string
		req  WechatMiniProgramRequest
	}{
		{
			name: "invalid phone",
			req: WechatMiniProgramRequest{
				Name:   "Alice",
				Phone:  "not-a-phone",
				AppID:  "wx-app",
				JsCode: "js-code",
			},
		},
		{
			name: "invalid email",
			req: WechatMiniProgramRequest{
				Name:   "Alice",
				Email:  "not-an-email",
				AppID:  "wx-app",
				JsCode: "js-code",
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.SignUpWithWechatMiniProgram(context.Background(), tc.req)

			require.Error(t, err)
			require.True(t, sdkerrors.IsInvalidArgument(err))
			require.False(t, called)
		})
	}
}
