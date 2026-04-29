package jwks

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ==================== PublicJWK 值对象 ====================
// PublicJWK 公钥 JWK 表示
type PublicJWK struct {
	Kty string `json:"kty"` // "RSA"/"EC"/"OKP"
	Use string `json:"use"` // "sig"
	Alg string `json:"alg"` // "RS256"/"ES256"/"EdDSA"
	Kid string `json:"kid"` // key id
	// RSA: n/e; EC: crv/x/y; OKP: crv/x
	N   *string `json:"n,omitempty"`
	E   *string `json:"e,omitempty"`
	Crv *string `json:"crv,omitempty"`
	X   *string `json:"x,omitempty"`
	Y   *string `json:"y,omitempty"`
}

// Validate 验证 PublicJWK 结构
func (p *PublicJWK) Validate() error {
	// 1. 基本字段验证
	if p.Kid == "" {
		return errors.WithCode(code.ErrInvalidKid, "kid cannot be empty")
	}
	if p.Kty == "" {
		return errors.WithCode(code.ErrInvalidJWK, "kty cannot be empty")
	}
	if p.Use != "sig" {
		return errors.WithCode(code.ErrInvalidJWKUse, "use must be 'sig'")
	}
	if p.Alg == "" {
		return errors.WithCode(code.ErrInvalidJWKAlg, "alg cannot be empty")
	}

	// 2. 根据 Kty 验证特定字段
	switch p.Kty {
	case "RSA":
		if p.N == nil || p.E == nil {
			return errors.WithCode(code.ErrMissingRSAParams, "n and e are required for RSA")
		}
	case "EC":
		if p.Crv == nil || p.X == nil || p.Y == nil {
			return errors.WithCode(code.ErrMissingECParams, "crv, x, y are required for EC")
		}
	case "OKP":
		if p.Crv == nil || p.X == nil {
			return errors.WithCode(code.ErrMissingOKPParams, "crv, x are required for OKP")
		}
	default:
		return errors.WithCode(code.ErrUnsupportedKty, "unsupported key type")
	}

	return nil
}

// ==================== JWKS 值对象 ====================
// JWKS 表示 JSON Web Key Set (RFC 7517)
type JWKS struct {
	Keys []PublicJWK `json:"keys"`
}

// Validate 验证 JWKS 结构
func (j *JWKS) Validate() error {
	if len(j.Keys) == 0 {
		return errors.WithCode(code.ErrEmptyJWKS, "JWKS cannot be empty")
	}
	// 验证每个 JWK
	for i, key := range j.Keys {
		if err := key.Validate(); err != nil {
			return errors.Wrapf(err, "JWKS validation failed at index %d", i)
		}
	}
	return nil
}

// FindByKid 根据 Kid 查找 JWK
func (j *JWKS) FindByKid(kid string) *PublicJWK {
	for i := range j.Keys {
		if j.Keys[i].Kid == kid {
			return &j.Keys[i]
		}
	}
	return nil
}

// Count 返回 JWK 数量
func (j *JWKS) Count() int {
	return len(j.Keys)
}

// IsEmpty 是否为空
func (j *JWKS) IsEmpty() bool {
	return len(j.Keys) == 0
}

// ==================== CacheTag 值对象 ====================

// CacheTag 缓存标签（用于 HTTP 缓存控制）
type CacheTag struct {
	ETag         string
	LastModified time.Time
}

// IsZero 是否为零值
func (c *CacheTag) IsZero() bool {
	return c.ETag == "" && c.LastModified.IsZero()
}

// Matches 是否匹配另一个缓存标签
func (c *CacheTag) Matches(other CacheTag) bool {
	return c.ETag == other.ETag
}

// GenerateETag 生成 ETag（基于内容哈希）
func GenerateETag(content []byte) string {
	hash := sha256.Sum256(content)
	return `"` + hex.EncodeToString(hash[:]) + `"`
}

// ==================== RotationPolicy 值对象 ====================

// RotationPolicy 密钥轮换策略
type RotationPolicy struct {
	RotationInterval time.Duration // 轮换间隔（如 30 天）
	GracePeriod      time.Duration // 宽限期（如 7 天）
	MaxKeysInJWKS    int           // JWKS 中最多保留密钥数（如 3 个）
}

// DefaultRotationPolicy 默认轮换策略
func DefaultRotationPolicy() RotationPolicy {
	return RotationPolicy{
		RotationInterval: 30 * 24 * time.Hour, // 30 天
		GracePeriod:      7 * 24 * time.Hour,  // 7 天
		MaxKeysInJWKS:    3,                   // 最多 3 个密钥
	}
}

// Validate 验证策略有效性
func (p *RotationPolicy) Validate() error {
	if p.RotationInterval <= 0 {
		return errors.WithCode(code.ErrInvalidRotationInterval, "rotation interval must be positive")
	}
	if p.GracePeriod <= 0 {
		return errors.WithCode(code.ErrInvalidGracePeriod, "grace period must be positive")
	}
	if p.MaxKeysInJWKS < 2 {
		return errors.WithCode(code.ErrInvalidMaxKeys, "max keys must be at least 2")
	}
	if p.GracePeriod >= p.RotationInterval {
		return errors.WithCode(code.ErrGracePeriodTooLong, "grace period must be shorter than rotation interval")
	}
	return nil
}
