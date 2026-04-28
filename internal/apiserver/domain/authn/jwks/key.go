package jwks

import (
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// KeyStatus 表示密钥状态
type KeyStatus uint8

const (
	KeyActive  KeyStatus = iota + 1 // 当前签名用 + 发布
	KeyGrace                        // 仅验签（并存期），发布
	KeyRetired                      // 已下线，不发布
)

// String 返回状态的字符串表示
func (s KeyStatus) String() string {
	switch s {
	case KeyActive:
		return "active"
	case KeyGrace:
		return "grace"
	case KeyRetired:
		return "retired"
	default:
		return "unknown"
	}
}

// Key 密钥实体（聚合根）
type Key struct {
	Kid       string
	Status    KeyStatus // 当前状态
	JWK       PublicJWK // 公钥 JWK 表示
	NotBefore *time.Time
	NotAfter  *time.Time
}

// ==================== 工厂方法 ====================

// NewKey 创建新密钥
func NewKey(kid string, jwk PublicJWK, opts ...KeyOption) *Key {
	key := &Key{
		Kid:    kid,
		Status: KeyActive, // 默认激活状态
		JWK:    jwk,
	}
	for _, opt := range opts {
		opt(key)
	}
	return key
}

// KeyOption 密钥选项
type KeyOption func(*Key)

// WithNotBefore 设置生效时间
func WithNotBefore(t time.Time) KeyOption {
	return func(k *Key) {
		k.NotBefore = &t
	}
}

// WithNotAfter 设置过期时间
func WithNotAfter(t time.Time) KeyOption {
	return func(k *Key) {
		k.NotAfter = &t
	}
}

// WithStatus 设置初始状态
func WithStatus(status KeyStatus) KeyOption {
	return func(k *Key) {
		k.Status = status
	}
}

// ==================== 状态查询 ====================

// IsActive 是否为激活状态（可签名+可验签+发布）
func (k *Key) IsActive() bool {
	return k.Status == KeyActive
}

// IsGrace 是否为宽限期（仅可验签+发布）
func (k *Key) IsGrace() bool {
	return k.Status == KeyGrace
}

// IsRetired 是否已退役（不发布）
func (k *Key) IsRetired() bool {
	return k.Status == KeyRetired
}

// ==================== 能力查询 ====================

// CanSign 是否可以用于签名
// 只有激活状态且未过期的密钥才能签名
func (k *Key) CanSign() bool {
	return k.IsActive() && !k.IsExpired(time.Now())
}

// CanVerify 是否可以用于验签
// 激活状态和宽限期的密钥都可以验签
func (k *Key) CanVerify() bool {
	return (k.IsActive() || k.IsGrace()) && !k.IsExpired(time.Now())
}

// ShouldPublish 是否应该发布到 JWKS
// 激活状态和宽限期的密钥会被发布到 /.well-known/jwks.json
func (k *Key) ShouldPublish() bool {
	return (k.IsActive() || k.IsGrace()) && !k.IsExpired(time.Now())
}

// ==================== 有效期检查 ====================

// IsExpired 是否已过期
func (k *Key) IsExpired(now time.Time) bool {
	if k.NotAfter != nil && now.After(*k.NotAfter) {
		return true
	}
	return false
}

// IsNotYetValid 是否尚未生效
func (k *Key) IsNotYetValid(now time.Time) bool {
	if k.NotBefore != nil && now.Before(*k.NotBefore) {
		return true
	}
	return false
}

// IsValidAt 在指定时间是否有效
func (k *Key) IsValidAt(t time.Time) bool {
	return !k.IsExpired(t) && !k.IsNotYetValid(t)
}

// ==================== 状态转换 ====================

// EnterGrace 进入宽限期（从 Active → Grace）
// 当新密钥激活时，旧密钥进入宽限期，在此期间仍可用于验签
func (k *Key) EnterGrace() error {
	if !k.IsActive() {
		return errors.WithCode(code.ErrInvalidStateTransition, "can only enter grace period from active state")
	}
	k.Status = KeyGrace
	return nil
}

// Retire 退役（从 Grace → Retired）
// 宽限期结束后，密钥正式退役，不再发布到 JWKS
func (k *Key) Retire() error {
	if !k.IsGrace() {
		return errors.WithCode(code.ErrInvalidStateTransition, "can only retire from grace period")
	}
	k.Status = KeyRetired
	return nil
}

// ForceRetire 强制退役（从任意状态 → Retired）
// 用于应急情况，如密钥泄露
func (k *Key) ForceRetire() {
	k.Status = KeyRetired
}

// ==================== 验证方法 ====================

// Validate 验证密钥完整性
func (k *Key) Validate() error {
	// 1. 验证 Kid
	if k.Kid == "" {
		return errors.WithCode(code.ErrInvalidKid, "kid cannot be empty")
	}

	// 2. 验证 JWK 基本字段
	if k.JWK.Kty == "" {
		return errors.WithCode(code.ErrInvalidJWK, "kty cannot be empty")
	}
	if k.JWK.Use != "sig" {
		return errors.WithCode(code.ErrInvalidJWKUse, "use must be 'sig'")
	}
	if k.JWK.Alg == "" {
		return errors.WithCode(code.ErrInvalidJWKAlg, "alg cannot be empty")
	}
	if k.JWK.Kid != k.Kid {
		return errors.WithCode(code.ErrKidMismatch, "key.Kid and JWK.Kid must be equal")
	}

	// 3. 根据 Kty 验证必需字段
	switch k.JWK.Kty {
	case "RSA":
		if k.JWK.N == nil || k.JWK.E == nil {
			return errors.WithCode(code.ErrMissingRSAParams, "n and e are required for RSA")
		}
	case "EC":
		if k.JWK.Crv == nil || k.JWK.X == nil || k.JWK.Y == nil {
			return errors.WithCode(code.ErrMissingECParams, "crv, x, y are required for EC")
		}
	case "OKP":
		if k.JWK.Crv == nil || k.JWK.X == nil {
			return errors.WithCode(code.ErrMissingOKPParams, "crv, x are required for OKP")
		}
	default:
		return errors.WithCode(code.ErrUnsupportedKty, "unsupported key type")
	}

	// 4. 验证有效期逻辑
	if k.NotBefore != nil && k.NotAfter != nil {
		if k.NotAfter.Before(*k.NotBefore) {
			return errors.WithCode(code.ErrInvalidTimeRange, "NotAfter must be after NotBefore")
		}
	}

	return nil
}
