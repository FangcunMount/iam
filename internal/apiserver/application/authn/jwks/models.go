package jwks

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// KeyStatus 表示签名密钥的生命周期状态。
type KeyStatus uint8

const (
	KeyActive  KeyStatus = iota + 1 // 当前签名用 + 发布
	KeyGrace                        // 仅验签（并存期），发布
	KeyRetired                      // 已下线，不发布
)

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

// PublicJWK 是 JWKS 对外发布的公钥表示。
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

// JWKS 表示 JSON Web Key Set。
type JWKS struct {
	Keys []PublicJWK `json:"keys"`
}

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

func (j *JWKS) FindByKid(kid string) *PublicJWK {
	for i := range j.Keys {
		if j.Keys[i].Kid == kid {
			return &j.Keys[i]
		}
	}
	return nil
}

func (j *JWKS) Count() int {
	return len(j.Keys)
}

func (j *JWKS) IsEmpty() bool {
	return len(j.Keys) == 0
}

// CacheTag 保存 JWKS 发布缓存标签。
type CacheTag struct {
	ETag         string
	LastModified time.Time
}

func (c *CacheTag) IsZero() bool {
	return c.ETag == "" && c.LastModified.IsZero()
}

func (c *CacheTag) Matches(other CacheTag) bool {
	return c.ETag == other.ETag
}

func GenerateETag(content []byte) string {
	hash := sha256.Sum256(content)
	return `"` + hex.EncodeToString(hash[:]) + `"`
}

// RotationPolicy 描述 JWKS signing key 的轮换策略。
type RotationPolicy struct {
	RotationInterval time.Duration
	GracePeriod      time.Duration
	MaxKeysInJWKS    int
}

func DefaultRotationPolicy() RotationPolicy {
	return RotationPolicy{
		RotationInterval: 30 * 24 * time.Hour,
		GracePeriod:      7 * 24 * time.Hour,
		MaxKeysInJWKS:    3,
	}
}

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

// ManagedKey 是 application 层暴露给密钥管理用例的签名密钥快照。
type ManagedKey struct {
	Kid       string
	Status    KeyStatus
	JWK       PublicJWK
	NotBefore *time.Time
	NotAfter  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewKey(kid string, jwk PublicJWK, opts ...KeyOption) *ManagedKey {
	now := time.Now()
	key := &ManagedKey{
		Kid:       kid,
		Status:    KeyActive,
		JWK:       jwk,
		CreatedAt: now,
		UpdatedAt: now,
	}
	for _, opt := range opts {
		opt(key)
	}
	return key
}

type KeyOption func(*ManagedKey)

func WithNotBefore(t time.Time) KeyOption {
	return func(k *ManagedKey) {
		k.NotBefore = &t
	}
}

func WithNotAfter(t time.Time) KeyOption {
	return func(k *ManagedKey) {
		k.NotAfter = &t
	}
}

func WithStatus(status KeyStatus) KeyOption {
	return func(k *ManagedKey) {
		k.Status = status
	}
}

func WithCreatedAt(t time.Time) KeyOption {
	return func(k *ManagedKey) {
		k.CreatedAt = t
	}
}

func WithUpdatedAt(t time.Time) KeyOption {
	return func(k *ManagedKey) {
		k.UpdatedAt = t
	}
}

type SnapshotStatus struct {
	Cached        bool
	KeyCount      int
	CacheTag      CacheTag
	LastBuildTime *time.Time
}
