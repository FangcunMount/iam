package authentication

import (
	"context"
	"time"
)

// ================== Interface Interfaces (Driving Ports) ==================
// 这些接口由领域层（领域服务）实现，供应用层调用
// 按照功能职责拆分，遵循接口隔离原则

// AuthCredential 认证凭据接口
type AuthCredential interface {
	// CredentialKind 返回认证凭据类型
	CredentialKind() CredentialKind
}

// AuthStrategy 认证策略（领域服务接口）
type AuthStrategy interface {
	// Kind 返回认证策略类型
	Kind() CredentialKind
	// Authenticate 执行认证
	// 参数：ctx 上下文, proof 认证凭据
	// 返回：认证决策, 错误
	// 职责：执行认证逻辑，返回认证决策
	Authenticate(ctx context.Context, proof AuthCredential) (AuthDecision, error)
}

// ================== External Service Interfaces (Driven Ports) ==================
// 定义领域模型所依赖的外部服务接口，由基础设施层提供实现

// PasswordHasher 密码哈希服务（密码加密算法）
// 职责：提供密码哈希验证和rehash能力
type PasswordHasher interface {
	// Verify 验证明文密码与存储的哈希值是否匹配
	// storedHash: PHC格式的哈希值，如 $argon2id$v=19$m=65536,t=3,p=4$...
	// plaintext: 明文密码
	Verify(storedHash, plaintext string) bool

	// NeedRehash 检查哈希值是否需要重新哈希（算法升级）
	NeedRehash(storedHash string) bool

	// Hash 对明文密码进行哈希
	Hash(plaintext string) (string, error)

	// Pepper 获取全局pepper（用于加盐）
	Pepper() string
}

// LoginPhoneOTPVerifier 登录短信验证码验证服务。
// 职责：验证登录 OTP 并消费（防止重放）。
type LoginPhoneOTPVerifier interface {
	// VerifyAndConsumeLoginPhoneOTP 验证登录 OTP 并标记为已使用。
	VerifyAndConsumeLoginPhoneOTP(ctx context.Context, phoneE164, code string) bool
}

// OTPCodeStore 写入待校验的 OTP（与 Redis OTP 消费实现使用相同的 key 约定）
type OTPCodeStore interface {
	// Put 写入验证码，TTL 到期后自动失效
	Put(ctx context.Context, phoneE164, scene, code string, ttl time.Duration) error
	// Delete 删除已写入的验证码（例如短信发送失败时回滚）
	Delete(ctx context.Context, phoneE164, scene, code string) error
}

// OTPSendGate 限制同一手机号、同一场景的发送频率
type OTPSendGate interface {
	// TryAcquire 若允许发送返回 true；冷却期内返回 false（未产生错误时表示频控）
	TryAcquire(ctx context.Context, phoneE164, scene string, cooldown time.Duration) (bool, error)
}

// OTPSendQuota 限制同一手机号、同一场景在固定时间窗口内的累计发送次数。
// dimension 仅用于区分不同窗口维度的计数器（如 "hourly"/"daily"）。
type OTPSendQuota interface {
	// TryConsume 在窗口内累计一次发送；返回 true 表示未超过 limit。
	// limit<=0 或 window<=0 视为不限制，直接返回 true。
	TryConsume(ctx context.Context, phoneE164, scene, dimension string, limit int, window time.Duration) (bool, error)
	// Rollback 回退一次计数（发送失败时使用），不应使计数低于 0。
	Rollback(ctx context.Context, phoneE164, scene, dimension string, window time.Duration) error
}

// SMSSender 登录 OTP 触达通道：实现通常为「投递 MQ / 事件」，由下游真正发短信，IAM 不直连运营商
type SMSSender interface {
	SendLoginOTP(ctx context.Context, phoneE164, code string) error
}

// IdentityProvider 身份提供商服务（OAuth/OIDC）
// 职责：与外部IdP交互，换取用户身份标识
type IdentityProvider interface {
	// ExchangeWxMinipCode 微信小程序 code 换 session
	// 参数：appID 小程序ID, appSecret 小程序密钥, jsCode 登录凭证
	// 返回：OpenID、UnionID（可选）
	ExchangeWxMinipCode(ctx context.Context, appID, appSecret, jsCode string) (openID, unionID string, err error)

	// ExchangeWxOpenCode 微信开放平台/网站应用扫码登录：code 换 openID/unionID
	// 参数：appID 网站应用 ID, appSecret 应用密钥, code 授权码
	// 返回：OpenID、UnionID（绑定开放平台后才有）
	ExchangeWxOpenCode(ctx context.Context, appID, appSecret, code string) (openID, unionID string, err error)

	// ExchangeWecomCode 企业微信 code 换 用户信息
	// 参数：corpID 企业ID, agentID 应用ID, corpSecret 应用密钥, code 登录凭证
	// 返回：OpenUserID、UserID
	ExchangeWecomCode(ctx context.Context, corpID, agentID, corpSecret, code string) (openUserID, userID string, err error)
}

// AuditLogger 审计日志（可选）
// 职责：记录认证事件（成功/失败/锁定）
type AuditLogger interface {
	// LogAuthAttempt 记录认证事件
	// 参数：ctx 上下文, event 认证事件
	LogAuthAttempt(ctx context.Context, event AuthAuditEvent)
}
