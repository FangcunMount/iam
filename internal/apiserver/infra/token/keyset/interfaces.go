package keyset

import (
	"context"
	"time"
)

// Reader is the read-only keyset surface exposed through the application adapter.
type Reader interface {
	GetActiveKey(ctx context.Context) (*Key, error)
	GetKeyByKid(ctx context.Context, kid string) (*Key, error)
	ListKeys(ctx context.Context, status KeyStatus, limit, offset int) ([]*Key, int64, error)
}

// Manager is the low-level keyset engine used by the lifecycle coordinator.
// Mutation methods are deliberately not exposed by the application reader adapter.
type Manager interface {
	Reader

	// CreateKey 创建新密钥
	// alg: 签名算法（RS256/ES256/EdDSA 等）
	// notBefore: 生效时间（可选，默认为当前时间）
	// notAfter: 过期时间（可选，根据 RotationPolicy 计算）
	// 返回：创建的 Key 实体
	CreateKey(ctx context.Context, alg string, notBefore, notAfter *time.Time) (*Key, error)

	// RetireKey 退役密钥（Grace → Retired）
	// 只能对 Grace 状态的密钥执行
	// kid: 密钥 ID
	RetireKey(ctx context.Context, kid string) error

	// ForceRetireKey 强制退役密钥（任何状态 → Retired）
	// 用于紧急情况（密钥泄露等）
	// kid: 密钥 ID
	ForceRetireKey(ctx context.Context, kid string) error

	// EnterGracePeriod 进入宽限期（Active → Grace）
	// 只能对 Active 状态的密钥执行
	// kid: 密钥 ID
	EnterGracePeriod(ctx context.Context, kid string) error

	// CleanupExpiredKeys 清理过期密钥
	// 删除 NotAfter < now 且 Status = Retired 的密钥
	// 返回：清理的密钥数量
	CleanupExpiredKeys(ctx context.Context) (int, error)
}

// Publisher JWKS 发布服务接口
// 负责构建和发布 /.well-known/jwks.json
// 由应用层调用，实现在 keyset 基础设施层。
type Publisher interface {
	// BuildJWKS 构建 JWKS JSON
	// 查询所有可发布的密钥（Active + Grace 状态且未过期）
	// 返回：JWKS JSON 字节流和缓存标签
	BuildJWKS(ctx context.Context) (jwksJSON []byte, tag CacheTag, err error)

	// GetPublishableKeys 获取可发布的密钥列表
	// 用于预览或调试
	GetPublishableKeys(ctx context.Context) ([]*Key, error)

	// ValidateCacheTag 验证缓存标签
	// 用于 HTTP 304 Not Modified 响应
	// clientTag: 客户端提供的 ETag 或 Last-Modified
	// 返回：true 表示缓存有效（未变更）
	ValidateCacheTag(ctx context.Context, clientTag CacheTag) (bool, error)

	// GetCurrentCacheTag 获取当前缓存标签
	// 用于生成 HTTP 响应头
	GetCurrentCacheTag(ctx context.Context) (CacheTag, error)

	// RefreshCache 刷新缓存
	// 用于强制更新缓存（密钥轮换后）
	RefreshCache(ctx context.Context) error
}

// Lifecycle is the canonical JWKS mutation boundary implemented by keyset.
type Lifecycle interface {
	CreateAndActivate(ctx context.Context, alg string, notBefore, notAfter *time.Time) (*Key, bool, error)
	RotateIfDue(ctx context.Context) (*Key, bool, error)
	RetireKey(ctx context.Context, kid string) error
	ForceRetireKey(ctx context.Context, kid string) error
	EnterGracePeriod(ctx context.Context, kid string) error
	CleanupExpiredKeys(ctx context.Context) (int, error)
}

// JWKSResponse JWKS HTTP 响应
type JWKSResponse struct {
	// JWKS JWKS 对象
	JWKS JWKS

	// CacheTag 缓存标签
	CacheTag CacheTag

	// MaxAge 缓存最大有效期（秒）
	MaxAge int
}
