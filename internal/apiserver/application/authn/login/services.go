package login

import (
	"context"

	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ============= 应用服务接口（Driving Ports）=============

// LoginApplicationService 是 transport 依赖的登录门面。
type LoginApplicationService interface {
	// Login 根据调用者输入选择登录方式，完成认证并签发令牌。
	Login(ctx context.Context, req LoginRequest) (*LoginResult, error)

	// Logout 撤销用户的访问令牌或刷新令牌，使其失效。
	Logout(ctx context.Context, req LogoutRequest) error
}

// ============= DTOs =============

// AuthType 是 v2 explicit 请求里用于选择登录方式的 wire value。
type AuthType string

const (
	AuthTypePassword AuthType = AuthType(authentication.AuthPassword) // 密码认证
	AuthTypePhoneOTP AuthType = AuthType(authentication.AuthPhoneOTP) // 手机号OTP认证
	// AuthTypeWechat 是 public wire value；它映射到 domain scenario oauth_wx_minip。
	AuthTypeWechat AuthType = "wechat"
	// AuthTypeWecom 是 public wire value；它映射到 domain scenario oauth_wecom。
	AuthTypeWecom    AuthType = "wecom"
	AuthTypeJWTToken AuthType = "jwt_token" // bearer-token compatibility method
)

// SignInSelectionMode 控制登录命令如何选择登录方式。
type SignInSelectionMode string

const (
	// SignInSelectionLegacy 保持 v1 旧行为：根据字段存在性推断登录方式，AuthType 不作为权威字段。
	SignInSelectionLegacy SignInSelectionMode = ""
	// SignInSelectionExplicit 用于 v2：AuthType 是权威字段，只读取对应 method payload 映射出的字段。
	SignInSelectionExplicit SignInSelectionMode = "explicit"
	// ScenarioSelectionLegacy 是旧命名兼容别名；新代码使用 SignInSelectionLegacy。
	ScenarioSelectionLegacy = SignInSelectionLegacy
	// ScenarioSelectionExplicit 是旧命名兼容别名；新代码使用 SignInSelectionExplicit。
	ScenarioSelectionExplicit = SignInSelectionExplicit
)

// ScenarioSelectionMode 是旧命名兼容别名；新代码使用 SignInSelectionMode。
type ScenarioSelectionMode = SignInSelectionMode

// SignInCommand 是登录用例的应用层输入。
type SignInCommand struct {
	// ========== 认证类型（必须）==========
	AuthType      AuthType            // 认证类型
	SelectionMode SignInSelectionMode // 登录方式选择模式；零值保持 v1 legacy 字段推断

	// ========== 密码认证字段 ==========
	TenantID meta.ID // 租户ID（可选）
	Username *string // 用户名（当 AuthType=password 时必须）
	Password *string // 密码（当 AuthType=password 时必须）

	// ========== 手机OTP认证字段 ==========
	PhoneE164 *string // E.164格式手机号（当 AuthType=phone_otp 时必须）
	OTPCode   *string // OTP验证码（当 AuthType=phone_otp 时必须）

	// ========== 微信小程序认证字段 ==========
	WechatAppID  *string // 微信AppID（当 AuthType=wechat 时必须）
	WechatJSCode *string // wx.login返回的code（当 AuthType=wechat 时必须）

	// ========== 企业微信认证字段 ==========
	WecomCorpID *string // 企业CorpID（当 AuthType=wecom 时必须）
	WecomCode   *string // 企业微信授权code（当 AuthType=wecom 时必须）

	// ========== JWT令牌认证字段 ==========
	JWTToken *string // bearer access token（当 AuthType=jwt_token 时必须）
}

// WecomConfig contains server-side credentials that are intentionally not
// exposed through REST/gRPC login requests.
type WecomConfig struct {
	AgentID string
}

// LoginRequest 保留给现有 transport 调用方；语义等同于 SignInCommand。
type LoginRequest = SignInCommand

// SignInResult 是登录用例的应用层输出。
type SignInResult struct {
	// 认证主体
	Principal *authentication.Principal // 认证主体信息

	// 令牌对
	TokenPair *tokenapp.TokenPair // 访问令牌 + 刷新令牌

	// 用户标识
	UserID    meta.ID // 用户ID
	AccountID meta.ID // 账户ID
	TenantID  meta.ID // 租户ID（可选）
}

// LoginResult 保留给现有 transport 调用方；语义等同于 SignInResult。
type LoginResult = SignInResult

// SignOutCommand 是登出用例的应用层输入。
type SignOutCommand struct {
	// AccessToken 或 RefreshToken 二选一
	// 如果提供 AccessToken，只撤销该访问令牌
	// 如果提供 RefreshToken，撤销刷新令牌（更彻底，会使所有通过该刷新令牌签发的访问令牌失效）
	AccessToken  *string // 访问令牌
	RefreshToken *string // 刷新令牌
}

// LogoutRequest 保留给现有 transport 调用方；语义等同于 SignOutCommand。
type LogoutRequest = SignOutCommand
