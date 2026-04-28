package authentication

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

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

// OTPVerifier OTP验证服务（一次性密码验证）
// 职责：验证OTP并消费（防止重放）
type OTPVerifier interface {
	// VerifyAndConsume 验证OTP并标记为已使用
	// scene: "login" | "register" | ...
	// 返回：是否验证通过
	VerifyAndConsume(ctx context.Context, phoneE164, scene, code string) bool
}

// OTPCodeStore 写入待校验的 OTP（与 OTPVerifier 使用相同的 Redis key 约定）
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

	// ExchangeWecomCode 企业微信 code 换 用户信息
	// 参数：corpID 企业ID, agentID 应用ID, corpSecret 应用密钥, code 登录凭证
	// 返回：OpenUserID、UserID
	ExchangeWecomCode(ctx context.Context, corpID, agentID, corpSecret, code string) (openUserID, userID string, err error)
}

// TokenVerifier JWT令牌验证服务
// 职责：验证JWT访问令牌的有效性
type TokenVerifier interface {
	// VerifyAccessToken 验证访问令牌
	// 返回：用户ID、账户ID、租户ID（可选）、错误信息
	// 如果令牌无效/过期/被撤销，返回错误
	VerifyAccessToken(ctx context.Context, tokenValue string) (userID, accountID meta.ID, tenantID meta.ID, err error)
}

// AuditLogger 审计日志（可选）
// 职责：记录认证事件（成功/失败/锁定）
type AuditLogger interface {
	LogAuthAttempt(ctx context.Context, event AuthAuditEvent)
}

type AuthAuditEvent struct {
	AccountID    meta.ID
	CredentialID meta.ID
	Scenario     string
	Success      bool
	ErrCode      string
	RemoteIP     string
	UserAgent    string
	Timestamp    time.Time
}
