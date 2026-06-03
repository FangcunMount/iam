package linking

import (
	"context"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// WechatOpenAuthorizeURLBuilder 构造微信开放平台扫码授权 URL。
type WechatOpenAuthorizeURLBuilder interface {
	BuildWebAppQRConnectURL(appID, redirectURI, state string) (string, error)
}

// StartWechatOpenLinkAuthorizeInput 发起微信开放平台扫码绑定授权。
//
// UserID 由 transport 从已验证 access token 注入，AppID/RedirectURI 由服务端配置注入，
// 均不得来自前端 payload。
type StartWechatOpenLinkAuthorizeInput struct {
	UserID      meta.ID
	AppID       string
	RedirectURI string
	Nonce       string
}

// StartWechatOpenLinkAuthorizeResult 返回 state 与跳转地址。
type StartWechatOpenLinkAuthorizeResult struct {
	State        string
	Nonce        string
	AuthorizeURL string
	ExpiresAt    time.Time
}

// StartWechatOpenLinkAuthorize 创建绑定场景 OAuth state 并生成微信授权 URL。
//
// 这是通用能力：任何已登录用户都可以发起绑定自己的微信开放平台身份，
// IAM 不关心调用方是哪个端，也不做产品准入检查。
type StartWechatOpenLinkAuthorize struct {
	states     WechatOpenLinkStateStarter
	urlBuilder WechatOpenAuthorizeURLBuilder
}

// NewStartWechatOpenLinkAuthorize 创建用例。
func NewStartWechatOpenLinkAuthorize(
	states WechatOpenLinkStateStarter,
	urlBuilder WechatOpenAuthorizeURLBuilder,
) *StartWechatOpenLinkAuthorize {
	return &StartWechatOpenLinkAuthorize{states: states, urlBuilder: urlBuilder}
}

// Execute 执行发起绑定授权。
func (u *StartWechatOpenLinkAuthorize) Execute(ctx context.Context, input StartWechatOpenLinkAuthorizeInput) (*StartWechatOpenLinkAuthorizeResult, error) {
	if u == nil || u.states == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat open link authorize use case is not initialized")
	}
	if u.urlBuilder == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wechat open authorize url builder is not configured")
	}
	if input.UserID.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "user_id is required")
	}

	appID := strings.TrimSpace(input.AppID)
	redirectURI := strings.TrimSpace(input.RedirectURI)
	created, err := u.states.StartWechatOpenLink(ctx, appID, redirectURI, input.UserID, strings.TrimSpace(input.Nonce))
	if err != nil {
		return nil, err
	}

	authorizeURL, err := u.urlBuilder.BuildWebAppQRConnectURL(appID, redirectURI, created.State)
	if err != nil {
		return nil, perrors.WithCode(code.ErrProofBuildFailed, "failed to build wechat authorize url: %v", err)
	}

	return &StartWechatOpenLinkAuthorizeResult{
		State:        created.State,
		Nonce:        created.Nonce,
		AuthorizeURL: authorizeURL,
		ExpiresAt:    created.ExpiresAt,
	}, nil
}
