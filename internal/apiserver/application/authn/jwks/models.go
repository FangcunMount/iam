package jwks

import (
	"time"
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

// JWKS 表示 JWKS 的 JSON 表示。
type JWKS struct {
	Keys []PublicJWK `json:"keys"`
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

// PublishableKey 表示公共 JWK Set 发布候选，不承载私钥管理能力。
type PublishableKey struct {
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
