package keyset

import (
	"context"
	"time"

	appjwks "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
	appsigningkey "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signingkey"
)

type applicationKeyManager struct {
	manager Reader
}

func NewApplicationKeyReader(manager Reader) appsigningkey.KeyReaderPort {
	return applicationKeyManager{manager: manager}
}

func (a applicationKeyManager) GetActiveKey(ctx context.Context) (*appsigningkey.ManagedKey, error) {
	key, err := a.manager.GetActiveKey(ctx)
	if err != nil {
		return nil, err
	}
	return toSigningKeyManagedKey(key), nil
}

func (a applicationKeyManager) GetKeyByKid(ctx context.Context, kid string) (*appsigningkey.ManagedKey, error) {
	key, err := a.manager.GetKeyByKid(ctx, kid)
	if err != nil {
		return nil, err
	}
	return toSigningKeyManagedKey(key), nil
}

func (a applicationKeyManager) ListKeys(ctx context.Context, status string, limit, offset int) ([]*appsigningkey.ManagedKey, int64, error) {
	keys, total, err := a.manager.ListKeys(ctx, keyStatusFromString(status), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return toSigningKeyManagedKeys(keys), total, nil
}

type applicationKeyPublisher struct {
	publisher Publisher
}

func NewApplicationKeyPublisher(publisher Publisher) appjwks.KeyPublisherPort {
	return applicationKeyPublisher{publisher: publisher}
}

func (a applicationKeyPublisher) BuildJWKS(ctx context.Context) ([]byte, appjwks.CacheTag, error) {
	jwksJSON, tag, err := a.publisher.BuildJWKS(ctx)
	return jwksJSON, toAppCacheTag(tag), err
}

func (a applicationKeyPublisher) GetPublishableKeys(ctx context.Context) ([]*appjwks.PublishableKey, error) {
	keys, err := a.publisher.GetPublishableKeys(ctx)
	if err != nil {
		return nil, err
	}
	return toAppPublishableKeys(keys), nil
}

func (a applicationKeyPublisher) ValidateCacheTag(ctx context.Context, clientTag appjwks.CacheTag) (bool, error) {
	return a.publisher.ValidateCacheTag(ctx, fromAppCacheTag(clientTag))
}

func (a applicationKeyPublisher) GetCurrentCacheTag(ctx context.Context) (appjwks.CacheTag, error) {
	tag, err := a.publisher.GetCurrentCacheTag(ctx)
	return toAppCacheTag(tag), err
}

func (a applicationKeyPublisher) RefreshCache(ctx context.Context) error {
	return a.publisher.RefreshCache(ctx)
}

type applicationKeyLifecycle struct {
	lifecycle Lifecycle
}

func NewApplicationKeyLifecycle(lifecycle Lifecycle) appsigningkey.KeyLifecyclePort {
	return applicationKeyLifecycle{lifecycle: lifecycle}
}

func (a applicationKeyLifecycle) CreateAndActivate(ctx context.Context, alg string, notBefore, notAfter *time.Time) (*appsigningkey.ManagedKey, bool, error) {
	key, changed, err := a.lifecycle.CreateAndActivate(ctx, alg, notBefore, notAfter)
	if err != nil {
		return nil, false, err
	}
	return toSigningKeyManagedKey(key), changed, nil
}

func (a applicationKeyLifecycle) RotateIfDue(ctx context.Context) (*appsigningkey.ManagedKey, bool, error) {
	key, rotated, err := a.lifecycle.RotateIfDue(ctx)
	if err != nil {
		return nil, false, err
	}
	return toSigningKeyManagedKey(key), rotated, nil
}

func (a applicationKeyLifecycle) RetireKey(ctx context.Context, kid string) error {
	return a.lifecycle.RetireKey(ctx, kid)
}

func (a applicationKeyLifecycle) ForceRetireKey(ctx context.Context, kid string) error {
	return a.lifecycle.ForceRetireKey(ctx, kid)
}

func (a applicationKeyLifecycle) CleanupExpiredKeys(ctx context.Context) (int, error) {
	return a.lifecycle.CleanupExpiredKeys(ctx)
}

type applicationLifecycleObserver struct{}

func NewApplicationLifecycleObserver() appsigningkey.LifecycleObserver {
	return applicationLifecycleObserver{}
}

func (applicationLifecycleObserver) RecordOperation(operation, result string) {
	recordLifecycleOperation(operation, result)
}

func (applicationLifecycleObserver) RecordPostCommitFailure(stage string) {
	recordPostCommitFailure(stage)
}

type snapshotReporter interface {
	SnapshotStatus() SnapshotStatus
}

type applicationSnapshotReporter struct {
	reporter snapshotReporter
}

func NewApplicationSnapshotReporter(reporter snapshotReporter) appjwks.SnapshotReporter {
	return applicationSnapshotReporter{reporter: reporter}
}

func (a applicationSnapshotReporter) SnapshotStatus() appjwks.SnapshotStatus {
	if a.reporter == nil {
		return appjwks.SnapshotStatus{}
	}
	return toAppSnapshotStatus(a.reporter.SnapshotStatus())
}
