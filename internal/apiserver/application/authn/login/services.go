package login

import (
	"context"

	tokenapp "github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ============= 应用服务接口（Driving Ports）=============

// LoginApplicationService 登录应用服务 - 统一的登录接口
type LoginApplicationService interface {
	// Login 统一登录接口
	// 根据 LoginRequest.AuthType 自动选择认证策略，完成认证并签发令牌
	Login(ctx context.Context, req LoginRequest) (*LoginResult, error)

	// Logout 登出接口
	// 撤销用户的访问令牌或刷新令牌，使其失效
	Logout(ctx context.Context, req LogoutRequest) error
}

// ============= DTOs =============

// AuthType 认证类型
type AuthType string

const (
	AuthTypePassword AuthType = "password"  // 密码认证
	AuthTypePhoneOTP AuthType = "phone_otp" // 手机号OTP认证
	AuthTypeWechat   AuthType = "wechat"    // 微信小程序认证
	AuthTypeWecom    AuthType = "wecom"     // 企业微信认证
	AuthTypeJWTToken AuthType = "jwt_token" // JWT令牌认证
)

// ScenarioSelectionMode 控制 LoginRequest 如何选择认证场景。
type ScenarioSelectionMode string

const (
	// ScenarioSelectionLegacy 保持 v1 旧行为：根据字段存在性推断认证场景，AuthType 不作为权威字段。
	ScenarioSelectionLegacy ScenarioSelectionMode = ""
	// ScenarioSelectionExplicit 用于 v2：AuthType 是权威字段，只读取对应 method payload 映射出的字段。
	ScenarioSelectionExplicit ScenarioSelectionMode = "explicit"
)

// LoginRequest 统一登录请求
type LoginRequest struct {
	// ========== 认证类型（必须）==========
	AuthType      AuthType              // 认证类型
	SelectionMode ScenarioSelectionMode // 场景选择模式；零值保持 v1 legacy 字段推断

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
	JWTToken *string // JWT访问令牌（当 AuthType=jwt_token 时必须）
}

// WecomConfig contains server-side credentials that are intentionally not
// exposed through REST/gRPC login requests.
type WecomConfig struct {
	AgentID string
}

// LoginResult 登录结果
type LoginResult struct {
	// 认证主体
	Principal *authentication.Principal // 认证主体信息

	// 令牌对
	TokenPair *tokenapp.TokenPair // 访问令牌 + 刷新令牌

	// 用户标识
	UserID    meta.ID // 用户ID
	AccountID meta.ID // 账户ID
	TenantID  meta.ID // 租户ID（可选）
}

// LogoutRequest 登出请求
type LogoutRequest struct {
	// AccessToken 或 RefreshToken 二选一
	// 如果提供 AccessToken，只撤销该访问令牌
	// 如果提供 RefreshToken，撤销刷新令牌（更彻底，会使所有通过该刷新令牌签发的访问令牌失效）
	AccessToken  *string // 访问令牌
	RefreshToken *string // 刷新令牌
}
