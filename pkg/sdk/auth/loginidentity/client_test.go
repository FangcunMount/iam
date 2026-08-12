package loginidentity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkerrors "github.com/FangcunMount/iam/v3/pkg/sdk/errors"
	"github.com/stretchr/testify/require"
)

func TestClientLinksWechatMiniProgram(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/authn/login-identities/wechat-miniprogram", r.URL.Path)
		require.Equal(t, "Bearer access-token", r.Header.Get("Authorization"))

		var req LinkWechatMiniProgramRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.Equal(t, "wx-app", req.AppID)
		require.Equal(t, "js-code", req.Code)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "success",
			"data": {
				"login_identity": {
					"id": "11",
					"provider": "wechat_minip",
					"realm": "wx-app",
					"identifier": "openid-1",
					"global_identifier": "union-1",
					"status": "active",
					"linked_at": "2026-05-10T10:00:00Z"
				},
				"reused": false
			}
		}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithBearerToken("access-token"))
	require.NoError(t, err)

	resp, err := client.LinkWechatMiniProgram(context.Background(), LinkWechatMiniProgramRequest{
		AppID: "wx-app",
		Code:  "js-code",
	})

	require.NoError(t, err)
	require.Equal(t, "11", resp.LoginIdentity.ID)
	require.Equal(t, "wechat_minip", resp.LoginIdentity.Provider)
	require.Equal(t, "union-1", resp.LoginIdentity.GlobalIdentifier)
	require.False(t, resp.Reused)
}

func TestClientListAndUnlink(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.Path] = true
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v2/authn/login-identities":
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"items":[{"id":"1","provider":"phone","realm":"global","identifier":"+8613800138000","status":"active","linked_at":"2026-05-10T10:00:00Z"}]}}`))
		case "DELETE /api/v2/authn/login-identities/1":
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"message":"login identity unlinked"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL + "/api/v2")
	require.NoError(t, err)

	list, err := client.List(context.Background())
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, "phone", list.Items[0].Provider)

	msg, err := client.Unlink(context.Background(), "1")
	require.NoError(t, err)
	require.Equal(t, "login identity unlinked", msg.Message)
	require.True(t, seen["GET /api/v2/authn/login-identities"])
	require.True(t, seen["DELETE /api/v2/authn/login-identities/1"])
}

func TestClientReturnsIAMErrorFromEnvelope(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":100033,"message":"login identity already exists"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	require.NoError(t, err)

	_, err = client.LinkPhone(context.Background(), LinkPhoneRequest{
		Phone:   "+8613800138000",
		OTPCode: "123456",
	})

	require.Error(t, err)
	require.True(t, sdkerrors.IsAlreadyExists(err))
	require.Equal(t, "login identity already exists", sdkerrors.Message(err))
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

	_, err = client.SendPhoneLinkChallenge(context.Background(), LinkPhoneChallengeRequest{})

	require.Error(t, err)
	require.False(t, called)
}
