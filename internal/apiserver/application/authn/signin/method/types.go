package method

import "github.com/FangcunMount/iam/v3/internal/pkg/meta"

// AuthMethod 是对外登录方式，来自 REST/gRPC auth_method。
type AuthMethod string

const (
	AuthMethodPassword   AuthMethod = "password"
	AuthMethodPhoneOTP   AuthMethod = "phone_otp"
	AuthMethodWechatMini AuthMethod = "wechat_mini"
	AuthMethodWechatScan AuthMethod = "wechat_scan"
	AuthMethodWecom      AuthMethod = "wecom"
)

// CredentialKind 是登录方式最终构造出的领域认证证明类型。
//
// 它是 application/method 层的选择结果字段。这里保留独立类型，是为了让
// AuthMethod（对外登录方式）与领域认证证明类型在应用层语义上分开；
// 真正构造领域 AuthCredential 时由 proof 层按该值选择 Builder。
type CredentialKind string

const (
	CredentialKindPassword    CredentialKind = "password"
	CredentialKindPhoneOTP    CredentialKind = "phone_otp"
	CredentialKindWechatMinip CredentialKind = "oauth_wx_minip"
	CredentialKindWechatScan  CredentialKind = "oauth_wx_scan"
	CredentialKindWecom       CredentialKind = "oauth_wecom"
)

// LoginMethod 表示一种可被选择的登录方式。
type LoginMethod interface {
	Method() AuthMethod
	CredentialKind() CredentialKind
	BuildPayload(LoginRequest) (Payload, error)
}

// LoginRequest 是 application login 用例的结构化请求。
//
// TenantID、RemoteIP、UserAgent 是请求上下文，必须由上游 transport /
// compatibility 装配到 LoginRequest 顶层字段。Payload 只允许携带具体登录
// 方式需要的字段，领域层和 proof 层不得再从 Payload 中读取这些公共上下文。
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
