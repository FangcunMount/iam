package signin

import (
	"context"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// WechatOpenAuthorizeURLBuilder 构造微信开放平台扫码授权 URL。
type WechatOpenAuthorizeURLBuilder interface {
	BuildWebAppQRConnectURL(appID, redirectURI, state string) (string, error)
}

// StartWechatOpenAuthorizeInput 发起微信扫码登录授权。
type StartWechatOpenAuthorizeInput struct {
	AppID       string
	RedirectURI string
	Nonce       string
}

// StartWechatOpenAuthorizeResult 返回 state 与跳转地址。
type StartWechatOpenAuthorizeResult struct {
	State        string
	Nonce        string
	AuthorizeURL string
	ExpiresAt    time.Time
}

// StartWechatOpenAuthorize 创建 OAuth state challenge 并生成微信授权 URL（T38）。
type StartWechatOpenAuthorize struct {
	oauthStates challengeApp.WechatOpenOAuthStateStarter
	urlBuilder  WechatOpenAuthorizeURLBuilder
}

// NewStartWechatOpenAuthorize 创建用例。
func NewStartWechatOpenAuthorize(
	oauthStates challengeApp.WechatOpenOAuthStateStarter,
	urlBuilder WechatOpenAuthorizeURLBuilder,
) *StartWechatOpenAuthorize {
	return &StartWechatOpenAuthorize{oauthStates: oauthStates, urlBuilder: urlBuilder}
}

// Execute 执行 StartWechatOpenAuthorize 用例。
func (u *StartWechatOpenAuthorize) Execute(ctx context.Context, input StartWechatOpenAuthorizeInput) (*StartWechatOpenAuthorizeResult, error) {
	if u == nil || u.oauthStates == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat open authorize use case is not initialized")
	}
	if u.urlBuilder == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat open authorize url builder is not configured")
	}

	appID := strings.TrimSpace(input.AppID)
	redirectURI := strings.TrimSpace(input.RedirectURI)
	created, err := u.oauthStates.StartWechatOpenLogin(ctx, challengeApp.StartWechatOpenLoginInput{
		AppID:       appID,
		RedirectURI: redirectURI,
		Nonce:       strings.TrimSpace(input.Nonce),
	})
	if err != nil {
		return nil, err
	}

	authorizeURL, err := u.urlBuilder.BuildWebAppQRConnectURL(appID, redirectURI, created.State)
	if err != nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "failed to build wechat authorize url: %v", err)
	}

	return &StartWechatOpenAuthorizeResult{
		State:        created.State,
		Nonce:        created.Nonce,
		AuthorizeURL: authorizeURL,
		ExpiresAt:    created.ExpiresAt,
	}, nil
}
