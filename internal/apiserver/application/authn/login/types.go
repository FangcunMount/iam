package login

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/proof"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type AuthMethod = method.AuthMethod

const (
	AuthMethodPassword = method.AuthMethodPassword
	AuthMethodPhoneOTP = method.AuthMethodPhoneOTP
	AuthMethodWechat   = method.AuthMethodWechat
	AuthMethodWecom    = method.AuthMethodWecom
)

type CredentialKind = method.CredentialKind
type LoginCommand = method.LoginRequest
type LoginRequest = LoginCommand
type LoginMethodSelection = method.LoginMethodSelection
type MethodPayload = method.Payload
type PasswordPayload = method.PasswordPayload
type PhoneOTPPayload = method.PhoneOTPPayload
type WechatMiniPayload = method.WechatPayload
type WecomPayload = method.WecomPayload
type WecomConfig = proof.WecomConfig

// SignInResult 登录结果
type SignInResult struct {
	Principal       *authentication.Principal
	TokenPair       *tokenapp.TokenPair
	UserID          meta.ID
	LoginIdentityID meta.ID
	TenantID        meta.ID
}

// LoginResult 登录结果
type LoginResult = SignInResult

// AuthResult 认证结果
type AuthResult struct {
	Principal       *authentication.Principal
	UserID          meta.ID
	LoginIdentityID meta.ID
	TenantID        meta.ID
}

// SignOutCommand 登出命令
type SignOutCommand struct {
	AccessToken  *string
	RefreshToken *string
}

// LogoutCommand 登出命令
type LogoutCommand = SignOutCommand

// LogoutRequest 登出请求
type LogoutRequest = LogoutCommand
