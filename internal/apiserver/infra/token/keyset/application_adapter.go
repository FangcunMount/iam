package keyset

import (
	"context"
	"time"

	appjwks "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/jwks"
)

type applicationKeyManager struct {
	manager Manager
}

func NewApplicationKeyManager(manager Manager) appjwks.KeyManagerPort {
	return applicationKeyManager{manager: manager}
}

func (a applicationKeyManager) CreateKey(ctx context.Context, alg string, notBefore, notAfter *time.Time) (*appjwks.ManagedKey, error) {
	key, err := a.manager.CreateKey(ctx, alg, notBefore, notAfter)
	if err != nil {
		return nil, err
	}
	return toAppManagedKey(key), nil
}

func (a applicationKeyManager) GetActiveKey(ctx context.Context) (*appjwks.ManagedKey, error) {
	key, err := a.manager.GetActiveKey(ctx)
	if err != nil {
		return nil, err
	}
	return toAppManagedKey(key), nil
}

func (a applicationKeyManager) GetKeyByKid(ctx context.Context, kid string) (*appjwks.ManagedKey, error) {
	key, err := a.manager.GetKeyByKid(ctx, kid)
	if err != nil {
		return nil, err
	}
	return toAppManagedKey(key), nil
}

func (a applicationKeyManager) RetireKey(ctx context.Context, kid string) error {
	return a.manager.RetireKey(ctx, kid)
}

func (a applicationKeyManager) ForceRetireKey(ctx context.Context, kid string) error {
	return a.manager.ForceRetireKey(ctx, kid)
}

func (a applicationKeyManager) EnterGracePeriod(ctx context.Context, kid string) error {
	return a.manager.EnterGracePeriod(ctx, kid)
}

func (a applicationKeyManager) CleanupExpiredKeys(ctx context.Context) (int, error) {
	return a.manager.CleanupExpiredKeys(ctx)
}

func (a applicationKeyManager) ListKeys(ctx context.Context, status appjwks.KeyStatus, limit, offset int) ([]*appjwks.ManagedKey, int64, error) {
	keys, total, err := a.manager.ListKeys(ctx, fromAppKeyStatus(status), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return toAppManagedKeys(keys), total, nil
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

func (a applicationKeyPublisher) GetPublishableKeys(ctx context.Context) ([]*appjwks.ManagedKey, error) {
	keys, err := a.publisher.GetPublishableKeys(ctx)
	if err != nil {
		return nil, err
	}
	return toAppManagedKeys(keys), nil
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

type applicationKeyRotator struct {
	rotator Rotator
}

func NewApplicationKeyRotator(rotator Rotator) appjwks.KeyRotatorPort {
	return applicationKeyRotator{rotator: rotator}
}

func (a applicationKeyRotator) RotateKey(ctx context.Context) (*appjwks.ManagedKey, error) {
	key, err := a.rotator.RotateKey(ctx)
	if err != nil {
		return nil, err
	}
	return toAppManagedKey(key), nil
}

func (a applicationKeyRotator) RotateIfDue(ctx context.Context) (*appjwks.ManagedKey, bool, error) {
	key, rotated, err := a.rotator.RotateIfDue(ctx)
	if err != nil {
		return nil, false, err
	}
	return toAppManagedKey(key), rotated, nil
}

func (a applicationKeyRotator) ShouldRotate(ctx context.Context) (bool, error) {
	return a.rotator.ShouldRotate(ctx)
}

func (a applicationKeyRotator) GetRotationPolicy() appjwks.RotationPolicy {
	return toAppRotationPolicy(a.rotator.GetRotationPolicy())
}

func (a applicationKeyRotator) UpdateRotationPolicy(ctx context.Context, policy appjwks.RotationPolicy) error {
	return a.rotator.UpdateRotationPolicy(ctx, fromAppRotationPolicy(policy))
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
