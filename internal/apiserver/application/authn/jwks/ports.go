package jwks

import (
	"context"
	"time"
)

type KeyManagerPort interface {
	CreateKey(ctx context.Context, alg string, notBefore, notAfter *time.Time) (*ManagedKey, error)
	GetActiveKey(ctx context.Context) (*ManagedKey, error)
	GetKeyByKid(ctx context.Context, kid string) (*ManagedKey, error)
	RetireKey(ctx context.Context, kid string) error
	ForceRetireKey(ctx context.Context, kid string) error
	EnterGracePeriod(ctx context.Context, kid string) error
	CleanupExpiredKeys(ctx context.Context) (int, error)
	ListKeys(ctx context.Context, status KeyStatus, limit, offset int) ([]*ManagedKey, int64, error)
}

type KeyPublisherPort interface {
	BuildJWKS(ctx context.Context) (jwksJSON []byte, tag CacheTag, err error)
	GetPublishableKeys(ctx context.Context) ([]*ManagedKey, error)
	ValidateCacheTag(ctx context.Context, clientTag CacheTag) (bool, error)
	GetCurrentCacheTag(ctx context.Context) (CacheTag, error)
	RefreshCache(ctx context.Context) error
}

type KeyRotatorPort interface {
	RotateKey(ctx context.Context) (*ManagedKey, error)
	RotateIfDue(ctx context.Context) (*ManagedKey, bool, error)
	ShouldRotate(ctx context.Context) (bool, error)
	GetRotationPolicy() RotationPolicy
	UpdateRotationPolicy(ctx context.Context, policy RotationPolicy) error
}

type SnapshotReporter interface {
	SnapshotStatus() SnapshotStatus
}
