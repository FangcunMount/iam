package signingkey

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestCreateAndActivateRefreshesOnlyAfterCommit(t *testing.T) {
	lifecycle := &keyLifecycleStub{
		key:     testManagedKey("key-new", "active"),
		changed: true,
	}
	publisher := &keyPublisherStub{}
	observer := &lifecycleObserverStub{}
	service := NewKeyLifecycleAppService(lifecycle, publisher, observer, log.New(log.NewOptions()))

	response, err := service.CreateAndActivate(context.Background(), CreateKeyRequest{Algorithm: "RS256"})
	require.NoError(t, err)
	require.Equal(t, "key-new", response.Kid)
	require.Equal(t, "active", response.Status)
	require.Equal(t, 1, publisher.refreshCalls)
	require.Equal(t, []string{"admin_create:success"}, observer.operations)

	lifecycle.err = errors.New("database transition failed")
	_, err = service.CreateAndActivate(context.Background(), CreateKeyRequest{Algorithm: "RS256"})
	require.Error(t, err)
	require.Equal(t, 1, publisher.refreshCalls, "failed mutation must not refresh")
	require.Equal(t, "admin_create:failed", observer.operations[len(observer.operations)-1])
}

func TestCommittedMutationSurvivesPublishRefreshFailure(t *testing.T) {
	lifecycle := &keyLifecycleStub{key: testManagedKey("key-new", "active"), changed: true}
	publisher := &keyPublisherStub{refreshErr: errors.New("refresh failed")}
	observer := &lifecycleObserverStub{}
	service := NewKeyLifecycleAppService(lifecycle, publisher, observer, log.New(log.NewOptions()))

	response, err := service.CreateAndActivate(context.Background(), CreateKeyRequest{Algorithm: "RS256"})
	require.NoError(t, err)
	require.Equal(t, "key-new", response.Kid)
	require.Equal(t, []string{"cache_refresh"}, observer.postCommitFailures)
}

func TestCreateAndActivateRejectsNonRS256Algorithm(t *testing.T) {
	lifecycle := &keyLifecycleStub{key: testManagedKey("key-new", "active"), changed: true}
	service := NewKeyLifecycleAppService(lifecycle, &keyPublisherStub{}, &lifecycleObserverStub{}, log.New(log.NewOptions()))

	response, err := service.CreateAndActivate(context.Background(), CreateKeyRequest{Algorithm: "RS384"})
	require.Error(t, err)
	require.Nil(t, response)
}

func TestRotateIfDueRefreshesOnlyWhenRotated(t *testing.T) {
	lifecycle := &keyLifecycleStub{key: testManagedKey("key-new", "active"), changed: true}
	publisher := &keyPublisherStub{}
	observer := &lifecycleObserverStub{}
	service := NewKeyLifecycleAppService(lifecycle, publisher, observer, log.New(log.NewOptions()))

	response, err := service.RotateIfDue(context.Background())
	require.NoError(t, err)
	require.True(t, response.Rotated)
	require.Equal(t, 1, publisher.refreshCalls)

	lifecycle.changed = false
	response, err = service.RotateIfDue(context.Background())
	require.NoError(t, err)
	require.False(t, response.Rotated)
	require.Equal(t, 1, publisher.refreshCalls)
	require.Equal(t, "auto_rotate:noop", observer.operations[len(observer.operations)-1])
}

func TestLifecycleMutationsRefreshPublishableState(t *testing.T) {
	lifecycle := &keyLifecycleStub{cleanupCount: 2}
	publisher := &keyPublisherStub{}
	service := NewKeyLifecycleAppService(lifecycle, publisher, &lifecycleObserverStub{}, log.New(log.NewOptions()))

	require.NoError(t, service.RetireKey(context.Background(), "retired"))
	require.NoError(t, service.ForceRetireKey(context.Background(), "forced"))
	result, err := service.CleanupExpiredKeys(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, result.DeletedCount)
	require.Equal(t, 3, publisher.refreshCalls)

	lifecycle.cleanupCount = 0
	_, err = service.CleanupExpiredKeys(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, publisher.refreshCalls, "empty cleanup is a no-op")
}

func testManagedKey(kid, status string) *ManagedKey {
	now := time.Now()
	return &ManagedKey{
		Kid:       kid,
		Algorithm: "RS256",
		Status:    status,
		JWK:       PublicJWK{Kid: kid, Alg: "RS256"},
		NotBefore: &now,
		CreatedAt: now,
	}
}

type keyLifecycleStub struct {
	key          *ManagedKey
	changed      bool
	err          error
	cleanupCount int
}

func (s *keyLifecycleStub) CreateAndActivate(context.Context, string, *time.Time, *time.Time) (*ManagedKey, bool, error) {
	return s.key, s.changed, s.err
}

func (s *keyLifecycleStub) RotateIfDue(context.Context) (*ManagedKey, bool, error) {
	return s.key, s.changed, s.err
}

func (s *keyLifecycleStub) RetireKey(context.Context, string) error {
	return s.err
}

func (s *keyLifecycleStub) ForceRetireKey(context.Context, string) error {
	return s.err
}

func (s *keyLifecycleStub) CleanupExpiredKeys(context.Context) (int, error) {
	return s.cleanupCount, s.err
}

type keyPublisherStub struct {
	refreshCalls int
	refreshErr   error
}

func (s *keyPublisherStub) RefreshCache(context.Context) error {
	s.refreshCalls++
	return s.refreshErr
}

type lifecycleObserverStub struct {
	operations         []string
	postCommitFailures []string
}

func (s *lifecycleObserverStub) RecordOperation(operation, result string) {
	s.operations = append(s.operations, operation+":"+result)
}

func (s *lifecycleObserverStub) RecordPostCommitFailure(stage string) {
	s.postCommitFailures = append(s.postCommitFailures, stage)
}
