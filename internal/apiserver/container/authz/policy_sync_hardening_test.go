package authz

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cbmessaging "github.com/FangcunMount/component-base/pkg/messaging"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
	"github.com/stretchr/testify/require"
)

type retrySubscriber struct {
	allowed atomic.Bool
	calls   atomic.Int32
	stops   atomic.Int32
}

func (s *retrySubscriber) Subscribe(string, string, cbmessaging.Handler) error {
	s.calls.Add(1)
	if !s.allowed.Load() {
		return fmt.Errorf("registration unavailable")
	}
	return nil
}
func (s *retrySubscriber) SubscribeWithMiddleware(topic, channel string, handler cbmessaging.Handler, _ ...cbmessaging.Middleware) error {
	return s.Subscribe(topic, channel, handler)
}
func (s *retrySubscriber) Stop()        { s.stops.Add(1) }
func (s *retrySubscriber) Close() error { return nil }

type lifecycleSource struct {
	version atomic.Int64
	block   atomic.Bool
	entered chan struct{}
	once    sync.Once
}

func (s *lifecycleSource) Load(context.Context) (authzruntime.Dataset, error) {
	return authzruntime.Dataset{Versions: map[string]int64{"a": s.version.Load()}}, nil
}
func (s *lifecycleSource) ReadVersions(ctx context.Context) (map[string]int64, error) {
	if s.block.Load() {
		s.once.Do(func() { close(s.entered) })
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return map[string]int64{"a": s.version.Load()}, nil
}
func TestPolicySyncRetriesRegistrationRestoresReadinessAndCancels(t *testing.T) {
	source := &lifecycleSource{entered: make(chan struct{})}
	runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(), authzruntime.WithConfig(authzruntime.Config{CheckInterval: 10 * time.Millisecond, SyncTimeout: 500 * time.Millisecond, MaxUnconfirmed: time.Second}))
	require.NoError(t, err)
	runtime.RequireSync()
	source.version.Store(2) // committed during the initial-load / subscription window; no event
	module := &AuthzModule{policyReloader: runtime, runtimeHealth: runtime}
	subscriber := &retrySubscriber{}
	syncer := module.PolicySyncSubscriber(subscriber)
	require.Same(t, syncer, module.PolicySyncSubscriber(subscriber))
	ready, _, _ := runtime.ReloadHealth()
	require.False(t, ready)
	require.NoError(t, syncer.Start(context.Background()))
	require.True(t, runtime.PolicyVersionLoaded("a", 2), "initial reconciliation must close startup gap")
	require.NoError(t, syncer.Start(context.Background()))
	ready, _, _ = runtime.ReloadHealth()
	require.False(t, ready)
	subscriber.allowed.Store(true)
	require.Eventually(t, func() bool { ready, _, _ := runtime.ReloadHealth(); return ready }, time.Second, 5*time.Millisecond)
	require.GreaterOrEqual(t, subscriber.calls.Load(), int32(2))
	source.block.Store(true)
	select {
	case <-source.entered:
	case <-time.After(time.Second):
		t.Fatal("version loop did not run")
	}
	start := time.Now()
	require.NoError(t, syncer.Stop())
	require.Less(t, time.Since(start), 250*time.Millisecond)
	require.NoError(t, syncer.Stop())
	require.EqualValues(t, 1, subscriber.stops.Load())
	ready, _, _ = runtime.ReloadHealth()
	require.False(t, ready)
}
