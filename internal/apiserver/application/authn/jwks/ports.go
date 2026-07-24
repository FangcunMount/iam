package jwks

import (
	"context"
	"time"
)

type KeyReaderPort interface {
	GetActiveKey(ctx context.Context) (*ManagedKey, error)
	GetKeyByKid(ctx context.Context, kid string) (*ManagedKey, error)
	ListKeys(ctx context.Context, status string, limit, offset int) ([]*ManagedKey, int64, error)
}

type KeyLifecyclePort interface {
	CreateAndActivate(ctx context.Context, alg string, notBefore, notAfter *time.Time) (*ManagedKey, bool, error)
	RotateIfDue(ctx context.Context) (*ManagedKey, bool, error)
	RetireKey(ctx context.Context, kid string) error
	ForceRetireKey(ctx context.Context, kid string) error
	EnterGracePeriod(ctx context.Context, kid string) error
	CleanupExpiredKeys(ctx context.Context) (int, error)
}

type KeyPublisherPort interface {
	BuildJWKS(ctx context.Context) (jwksJSON []byte, tag CacheTag, err error)
	GetPublishableKeys(ctx context.Context) ([]*ManagedKey, error)
	ValidateCacheTag(ctx context.Context, clientTag CacheTag) (bool, error)
	GetCurrentCacheTag(ctx context.Context) (CacheTag, error)
	RefreshCache(ctx context.Context) error
}

type LifecycleObserver interface {
	RecordOperation(operation, result string)
	RecordPostCommitFailure(stage string)
}

type SnapshotReporter interface {
	SnapshotStatus() SnapshotStatus
}
