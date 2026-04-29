package jwks

import (
	"context"
	"time"
)

// ================== Domain Service Interfaces (Driving Ports) ==================
// 这些接口由领域层（领域服务）实现，供应用层调用
// 按照功能职责拆分，遵循接口隔离原则

// Manager 密钥管理服务接口
// 负责密钥的生命周期管理：创建、激活、宽限、退役、清理
// 由应用层调用，实现在领域服务层
type Manager interface {
	// CreateKey 创建新密钥
	// alg: 签名算法（RS256/ES256/EdDSA 等）
	// notBefore: 生效时间（可选，默认为当前时间）
	// notAfter: 过期时间（可选，根据 RotationPolicy 计算）
	// 返回：创建的 Key 实体
	CreateKey(ctx context.Context, alg string, notBefore, notAfter *time.Time) (*Key, error)

	// GetActiveKey 获取当前激活的密钥
	// 用于 Token 签名时获取当前可用的密钥
	// 返回：当前 Active 状态且未过期的密钥
	GetActiveKey(ctx context.Context) (*Key, error)

	// GetKeyByKid 根据 kid 获取密钥
	// 用于验证 Token 时根据 kid 查找公钥
	GetKeyByKid(ctx context.Context, kid string) (*Key, error)

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

	// ListKeys 列出密钥（分页）
	// status: 可选的状态过滤（"", "active", "grace", "retired"）
	// limit/offset: 分页参数
	ListKeys(ctx context.Context, status KeyStatus, limit, offset int) ([]*Key, int64, error)
}

// Publisher JWKS 发布服务接口
// 负责构建和发布 /.well-known/jwks.json
// 由应用层调用，实现在领域服务层
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

// Rotator 密钥轮换服务接口
// 负责密钥的自动轮换：生成新密钥、旧密钥进入宽限期、清理过期密钥
// 由应用层调用（定时任务或手动触发），实现在领域服务层
type Rotator interface {
	// RotateKey 执行密钥轮换
	// 轮换流程：
	// 1. 生成新密钥（Active 状态）
	// 2. 将当前 Active 密钥转为 Grace 状态
	// 3. 清理超过 MaxKeys 的密钥（将最老的 Grace 密钥转为 Retired）
	// 4. 清理过期的 Retired 密钥
	// 返回：新生成的密钥
	RotateKey(ctx context.Context) (*Key, error)

	// ShouldRotate 判断是否需要轮换
	// 根据 RotationPolicy 判断当前 Active 密钥是否已到轮换时间
	// 返回：true 表示需要轮换
	ShouldRotate(ctx context.Context) (bool, error)

	// GetRotationPolicy 获取当前轮换策略
	GetRotationPolicy() RotationPolicy

	// UpdateRotationPolicy 更新轮换策略
	UpdateRotationPolicy(ctx context.Context, policy RotationPolicy) error
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

// RotationStatus 轮换状态
type RotationStatus struct {
	// LastRotation 上次轮换时间
	LastRotation time.Time

	// NextRotation 下次计划轮换时间
	NextRotation time.Time

	// ActiveKey 当前激活的密钥信息
	ActiveKey *KeyInfo

	// GraceKeys 宽限期密钥列表
	GraceKeys []*KeyInfo

	// RetiredKeys 已退役密钥数量
	RetiredKeys int

	// Policy 当前轮换策略
	Policy RotationPolicy
}

// KeyInfo 密钥信息摘要
type KeyInfo struct {
	Kid       string
	Status    KeyStatus
	Algorithm string
	NotBefore *time.Time
	NotAfter  *time.Time
	CreatedAt time.Time
}
