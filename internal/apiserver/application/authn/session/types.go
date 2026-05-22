package session

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/proof"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
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

type SignInResult = signin.Result
type LoginResult = signin.Result

// SignOutCommand 登出命令。
type SignOutCommand struct {
	AccessToken  *string
	RefreshToken *string
}

type LogoutCommand = SignOutCommand
type LogoutRequest = SignOutCommand

// RenewResult 会话续期结果。
type RenewResult = tokenapp.TokenRefreshResult

func PublicAuthMethods() []AuthMethod {
	return method.PublicAuthMethods()
}

func IsPublicAuthMethod(raw string) bool {
	return method.IsPublicAuthMethod(raw)
}
