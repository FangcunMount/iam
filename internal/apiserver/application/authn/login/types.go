package login

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/proof"
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

type SignInResult = signin.Result
type LoginResult = signin.Result

// SignOutCommand 登出命令。
type SignOutCommand struct {
	AccessToken  *string
	RefreshToken *string
}

type LogoutCommand = SignOutCommand
type LogoutRequest = SignOutCommand

// AuthResult 认证结果。
type AuthResult struct {
	Principal       *authentication.Principal
	UserID          meta.ID
	LoginIdentityID meta.ID
	TenantID        meta.ID
}

func PublicAuthMethods() []AuthMethod {
	return method.PublicAuthMethods()
}

func IsPublicAuthMethod(raw string) bool {
	return method.IsPublicAuthMethod(raw)
}

// authResultFromPrincipal 由 Principal 构造认证结果
// 参数：principal 认证主体
// 返回：认证结果
// 职责：由认证主体构造认证结果
func authResultFromPrincipal(principal *authentication.Principal) *AuthResult {
	if principal == nil {
		return &AuthResult{}
	}
	return &AuthResult{
		Principal:       principal,
		UserID:          principal.UserID,
		LoginIdentityID: principal.LoginIdentityID,
		TenantID:        principal.TenantID,
	}
}
