package keyset

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type KeyStatus uint8

const (
	KeyActive KeyStatus = iota + 1
	KeyGrace
	KeyRetired
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

type Key struct {
	Kid       string
	Status    KeyStatus
	JWK       PublicJWK
	NotBefore *time.Time
	NotAfter  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewKey(kid string, jwk PublicJWK, opts ...KeyOption) *Key {
	now := time.Now()
	key := &Key{
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

type KeyOption func(*Key)

func WithNotBefore(t time.Time) KeyOption {
	return func(k *Key) {
		k.NotBefore = &t
	}
}

func WithNotAfter(t time.Time) KeyOption {
	return func(k *Key) {
		k.NotAfter = &t
	}
}

func WithStatus(status KeyStatus) KeyOption {
	return func(k *Key) {
		k.Status = status
	}
}

func (k *Key) IsActive() bool {
	return k.Status == KeyActive
}

func (k *Key) IsGrace() bool {
	return k.Status == KeyGrace
}

func (k *Key) IsRetired() bool {
	return k.Status == KeyRetired
}

func (k *Key) CanSign() bool {
	return k.IsActive() && !k.IsExpired(time.Now())
}

func (k *Key) CanVerify() bool {
	return (k.IsActive() || k.IsGrace()) && !k.IsExpired(time.Now())
}

func (k *Key) ShouldPublish() bool {
	return (k.IsActive() || k.IsGrace()) && !k.IsExpired(time.Now())
}

func (k *Key) IsExpired(now time.Time) bool {
	return k.NotAfter != nil && now.After(*k.NotAfter)
}

func (k *Key) IsNotYetValid(now time.Time) bool {
	return k.NotBefore != nil && now.Before(*k.NotBefore)
}

func (k *Key) IsValidAt(t time.Time) bool {
	return !k.IsExpired(t) && !k.IsNotYetValid(t)
}

func (k *Key) EnterGrace() error {
	if !k.IsActive() {
		return errors.WithCode(code.ErrInvalidStateTransition, "can only enter grace period from active state")
	}
	k.Status = KeyGrace
	k.UpdatedAt = time.Now()
	return nil
}

func (k *Key) Retire() error {
	if !k.IsGrace() {
		return errors.WithCode(code.ErrInvalidStateTransition, "can only retire from grace period")
	}
	k.Status = KeyRetired
	k.UpdatedAt = time.Now()
	return nil
}

func (k *Key) ForceRetire() {
	k.Status = KeyRetired
	k.UpdatedAt = time.Now()
}

func (k *Key) Validate() error {
	if k.Kid == "" {
		return errors.WithCode(code.ErrInvalidKid, "kid cannot be empty")
	}
	if err := k.JWK.Validate(); err != nil {
		return err
	}
	if k.JWK.Kid != k.Kid {
		return errors.WithCode(code.ErrKidMismatch, "key.Kid and JWK.Kid must be equal")
	}
	if k.NotBefore != nil && k.NotAfter != nil && k.NotAfter.Before(*k.NotBefore) {
		return errors.WithCode(code.ErrInvalidTimeRange, "NotAfter must be after NotBefore")
	}
	return nil
}

type SnapshotStatus struct {
	Cached        bool
	KeyCount      int
	CacheTag      CacheTag
	LastBuildTime *time.Time
}
