package authz

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	sdktransport "github.com/FangcunMount/reliable-messaging/transport"
	"github.com/FangcunMount/reliable-messaging/wire/legacy"
	"github.com/stretchr/testify/require"
)

type retrySubscriber struct {
	allowed atomic.Bool
	calls   atomic.Int32
	stops   atomic.Int32
}

func (s *retrySubscriber) Subscribe(context.Context, string, string, sdktransport.Handler, func(context.Context, legacy.FailedHandoff) error) error {
	s.calls.Add(1)
	if !s.allowed.Load() {
		return fmt.Errorf("registration unavailable")
	}
	return nil
}
func (s *retrySubscriber) Close(context.Context) error { s.stops.Add(1); return nil }

type lifecycleSource struct {
	version atomic.Int64
	block   atomic.Bool
	entered chan struct{}
	once    sync.Once
}

func (s *lifecycleSource) Load(context.Context) (authzruntime.Dataset, error) {
	return authzruntime.Dataset{Version: s.version.Load()}, nil
}
func (s *lifecycleSource) ReadVersion(ctx context.Context) (int64, error) {
	if s.block.Load() {
		s.once.Do(func() { close(s.entered) })
		<-ctx.Done()
		return 0, ctx.Err()
	}
	return s.version.Load(), nil
}
func TestPolicySyncRetriesRegistrationRestoresReadinessAndCancels(t *testing.T) {
	source := &lifecycleSource{entered: make(chan struct{})}
	runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(), authzruntime.WithConfig(authzruntime.Config{CheckInterval: 10 * time.Millisecond, SyncTimeout: 500 * time.Millisecond, MaxUnconfirmed: time.Second}))
	require.NoError(t, err)
	runtime.RequireSync()
	source.version.Store(2) // committed during the initial-load / subscription window; no event
	module := &AuthzModule{policyReloader: runtime, runtimeHealth: runtime}
	subscriber := &retrySubscriber{}
	syncer := module.newSDKPolicySyncSubscriber(subscriber, policyProvisionerStub{}, ignorePolicyFailure)
	require.Same(t, syncer, module.newSDKPolicySyncSubscriber(subscriber, policyProvisionerStub{}, ignorePolicyFailure))
	ready, _, _ := runtime.ReloadHealth()
	require.False(t, ready)
	require.NoError(t, syncer.Start(context.Background()))
	require.True(t, runtime.PolicyVersionLoaded(2), "initial reconciliation must close startup gap")
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
