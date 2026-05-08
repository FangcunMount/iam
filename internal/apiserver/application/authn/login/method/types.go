package method

import (
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// AuthMethod 是对外登录方式，来自 REST/gRPC auth_method。
type AuthMethod string

const (
	AuthMethodPassword AuthMethod = AuthMethod(credDomain.CredPassword)
	AuthMethodPhoneOTP AuthMethod = AuthMethod(credDomain.CredPhoneOTP)
	AuthMethodWechat   AuthMethod = "wechat"
	AuthMethodWecom    AuthMethod = "wecom"
)

// CredentialKind 是登录方式最终构造出的领域凭据类型。
type CredentialKind string

const (
	CredentialKindPassword    CredentialKind = CredentialKind(credDomain.CredPassword)
	CredentialKindPhoneOTP    CredentialKind = CredentialKind(credDomain.CredPhoneOTP)
	CredentialKindWechatMinip CredentialKind = CredentialKind(credDomain.CredOAuthWxMinip)
	CredentialKindWecom       CredentialKind = CredentialKind(credDomain.CredOAuthWecom)
)

// LoginMethod 表示一种可被选择的登录方式。
type LoginMethod interface {
	Method() AuthMethod
	CredentialKind() CredentialKind
	BuildPayload(LoginRequest) (Payload, error)
}

// LoginRequest 是 application login 用例的结构化请求。
type LoginRequest struct {
	AuthMethod AuthMethod
	TenantID   meta.ID
	RemoteIP   string
	UserAgent  string
	Payload    Payload
}

// LoginMethodSelection 是登录方式选择后的结果。
type LoginMethodSelection struct {
	AuthMethod     AuthMethod
	CredentialKind CredentialKind
	Common         CommonPayload
	Payload        Payload
}

// TenantID 返回所选登录请求上下文中的租户ID。
func (s LoginMethodSelection) TenantID() meta.ID {
	return s.Common.TenantID
}
