package jwks

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// PublicJWK 表示 JWKS 对外发布的公钥。
type PublicJWK struct {
	Kty string  `json:"kty"`
	Use string  `json:"use"`
	Alg string  `json:"alg"`
	Kid string  `json:"kid"`
	N   *string `json:"n,omitempty"`
	E   *string `json:"e,omitempty"`
	Crv *string `json:"crv,omitempty"`
	X   *string `json:"x,omitempty"`
	Y   *string `json:"y,omitempty"`
}

// Validate 验证 PublicJWK 是否有效。
func (p *PublicJWK) Validate() error {
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

// JWKS 表示 JWKS 的 JSON 表示。
type JWKS struct {
	Keys []PublicJWK `json:"keys"`
}

// Validate 验证 JWKS 是否有效。
func (j *JWKS) Validate() error {
	if len(j.Keys) == 0 {
		return errors.WithCode(code.ErrEmptyJWKS, "JWKS cannot be empty")
	}
	for i, key := range j.Keys {
		if err := key.Validate(); err != nil {
			return errors.Wrapf(err, "JWKS validation failed at index %d", i)
		}
	}
	return nil
}

// FindByKid 根据 kid 查找 PublicJWK。
func (j *JWKS) FindByKid(kid string) *PublicJWK {
	for i := range j.Keys {
		if j.Keys[i].Kid == kid {
			return &j.Keys[i]
		}
	}
	return nil
}

// Count 返回 JWKS 中的公钥数量。
func (j *JWKS) Count() int {
	return len(j.Keys)
}

// IsEmpty 判断 JWKS 是否为空。
func (j *JWKS) IsEmpty() bool {
	return len(j.Keys) == 0
}

// CacheTag 表示 JWKS 发布缓存标签。
type CacheTag struct {
	ETag         string
	LastModified time.Time
}

// IsZero 判断 CacheTag 是否为空。
func (c *CacheTag) IsZero() bool {
	return c.ETag == "" && c.LastModified.IsZero()
}

// Matches 判断 CacheTag 是否与另一个 CacheTag 匹配。
func (c *CacheTag) Matches(other CacheTag) bool {
	return c.ETag == other.ETag
}

// GenerateETag 生成 ETag。
func GenerateETag(content []byte) string {
	hash := sha256.Sum256(content)
	return `"` + hex.EncodeToString(hash[:]) + `"`
}

// ManagedKey 表示密钥管理用例的签名密钥快照。
type ManagedKey struct {
	Kid       string
	Status    string
	JWK       PublicJWK
	NotBefore *time.Time
	NotAfter  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SnapshotStatus 表示 JWKS 快照的状态。
type SnapshotStatus struct {
	Cached        bool
	KeyCount      int
	CacheTag      CacheTag
	LastBuildTime *time.Time
}
