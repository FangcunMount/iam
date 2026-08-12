package authn

import (
	"context"

	challengeApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/challenge"
	jwksApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
	linkingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/linking"
	sessionApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin"
	signupApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signup"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
)

// KeyRotationScheduler is the runtime capability exposed by the authn module.
type KeyRotationScheduler interface {
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool
	TriggerNow(ctx context.Context) error
}

// ApplicationCapabilities contains authn application collaborators used
// by transports without exposing concrete transport objects from the module.
type ApplicationCapabilities struct {
	SignupService                signupApp.SignupService
	LoginIdentityLinking         linkingApp.Linker
	SessionService               sessionApp.ApplicationService
	SessionRevoker               sessionApp.Revoker
	LoginPhoneOTPSender          challengeApp.LoginPhoneOTPSender
	PhoneLinkOTPSender           challengeApp.PhoneLinkOTPSender
	StartWechatOpenAuthorize     *signin.StartWechatOpenAuthorize
	StartWechatOpenLinkAuthorize *linkingApp.StartWechatOpenLinkAuthorize
	CompleteWechatOpenLink       *linkingApp.CompleteWechatOpenLink
	WechatOpen                   WechatOpenConfig
	TokenService                 token.TokenApplicationService
	KeyManagementApp             *jwksApp.KeyManagementAppService
	KeyPublishApp                *jwksApp.KeyPublishAppService
	KeyLifecycleApp              *jwksApp.KeyLifecycleAppService
}

// RuntimeCapabilities exposes background collaborators owned by authn.
type RuntimeCapabilities struct {
	RotationScheduler KeyRotationScheduler
}

// WechatOpenConfig 暴露微信开放平台扫码登录/绑定所需的服务端配置（不来自前端）。
// 登录与绑定使用不同的回调地址。
type WechatOpenConfig struct {
	AppID            string
	LoginRedirectURI string
	LinkRedirectURI  string
}
