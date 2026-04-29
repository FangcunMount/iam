package keyset

import (
	"time"

	appjwks "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
)

type Key = appjwks.ManagedKey
type KeyStatus = appjwks.KeyStatus
type PublicJWK = appjwks.PublicJWK
type JWKS = appjwks.JWKS
type CacheTag = appjwks.CacheTag
type RotationPolicy = appjwks.RotationPolicy
type SnapshotStatus = appjwks.SnapshotStatus

const (
	KeyActive  = appjwks.KeyActive
	KeyGrace   = appjwks.KeyGrace
	KeyRetired = appjwks.KeyRetired
)

func DefaultRotationPolicy() RotationPolicy {
	return appjwks.DefaultRotationPolicy()
}

func GenerateETag(content []byte) string {
	return appjwks.GenerateETag(content)
}

// NewKey creates a managed signing key with active status by default.
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

func WithCreatedAt(t time.Time) KeyOption {
	return func(k *Key) {
		k.CreatedAt = t
	}
}

func WithUpdatedAt(t time.Time) KeyOption {
	return func(k *Key) {
		k.UpdatedAt = t
	}
}
