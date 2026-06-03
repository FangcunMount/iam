package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
)

type wechatOpenStateStarterStub struct {
	gotInput challengeApp.StartWechatOpenLoginInput
}

func (s *wechatOpenStateStarterStub) StartWechatOpenLogin(_ context.Context, input challengeApp.StartWechatOpenLoginInput) (*challengeApp.StartWechatOpenLoginResult, error) {
	s.gotInput = input
	return &challengeApp.StartWechatOpenLoginResult{
		State:     "state-123",
		Nonce:     input.Nonce,
		ExpiresAt: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
	}, nil
}

type wechatOpenURLBuilderStub struct {
	gotAppID       string
	gotRedirectURI string
	gotState       string
}

func (b *wechatOpenURLBuilderStub) BuildWebAppQRConnectURL(appID, redirectURI, state string) (string, error) {
	b.gotAppID = appID
	b.gotRedirectURI = redirectURI
	b.gotState = state
	return "https://open.weixin.qq.com/connect/qrconnect?appid=" + appID + "&state=" + state, nil
}

func TestWechatOpenLoginAuthorizeHandlerInjectsServerConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	starter := &wechatOpenStateStarterStub{}
	urlBuilder := &wechatOpenURLBuilderStub{}
	authorize := signin.NewStartWechatOpenAuthorize(starter, urlBuilder)
	h := NewWechatOpenLoginAuthorizeHandler(authorize, "wx-open-app", "https://host/login/callback")

	w := performAuthRequest(h.StartAuthorize, `{"nonce":"n-1"}`)

	require.Equal(t, http.StatusOK, w.Code)

	// app_id 与 redirect_uri 必须来自服务端配置，而非请求体
	require.Equal(t, "wx-open-app", starter.gotInput.AppID)
	require.Equal(t, "https://host/login/callback", starter.gotInput.RedirectURI)
	require.Equal(t, "n-1", starter.gotInput.Nonce)
	require.Equal(t, "wx-open-app", urlBuilder.gotAppID)
	require.Equal(t, "https://host/login/callback", urlBuilder.gotRedirectURI)
	require.Equal(t, "state-123", urlBuilder.gotState)

	var resp struct {
		Data struct {
			State        string `json:"state"`
			Nonce        string `json:"nonce"`
			AppID        string `json:"app_id"`
			AuthorizeURL string `json:"authorize_url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "state-123", resp.Data.State)
	require.Equal(t, "n-1", resp.Data.Nonce)
	require.Equal(t, "wx-open-app", resp.Data.AppID)
	require.Contains(t, resp.Data.AuthorizeURL, "state=state-123")
}

func TestWechatOpenLoginAuthorizeHandlerAllowsEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	starter := &wechatOpenStateStarterStub{}
	authorize := signin.NewStartWechatOpenAuthorize(starter, &wechatOpenURLBuilderStub{})
	h := NewWechatOpenLoginAuthorizeHandler(authorize, "wx-open-app", "https://host/login/callback")

	w := performAuthRequest(h.StartAuthorize, ``)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "wx-open-app", starter.gotInput.AppID)
	require.Empty(t, starter.gotInput.Nonce)
}
