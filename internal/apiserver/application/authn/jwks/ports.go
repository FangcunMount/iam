package jwks

import "context"

type KeyPublisherPort interface {
	BuildJWKS(ctx context.Context) (jwksJSON []byte, tag CacheTag, err error)
	GetPublishableKeys(ctx context.Context) ([]*PublishableKey, error)
	ValidateCacheTag(ctx context.Context, clientTag CacheTag) (bool, error)
	GetCurrentCacheTag(ctx context.Context) (CacheTag, error)
	RefreshCache(ctx context.Context) error
}

type SnapshotReporter interface {
	SnapshotStatus() SnapshotStatus
}
