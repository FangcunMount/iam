package runtime_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/assessment"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/stretchr/testify/require"
)

func TestReloadRejectsTenantVersionRegression(t *testing.T) {
	source := &mutableSource{dataset: assessmentDataset(t)}
	runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(), authzruntime.WithAttributeProviders(authzfixture.Policy()))
	require.NoError(t, err)
	source.dataset.Versions = map[string]int64{"fangcun": 8, "another": 100}
	require.Error(t, runtime.LoadPolicy(context.Background()))
	decision, err := runtime.Check(context.Background(), checkRequest(t, 2, "retry", "adhoc"))
	require.NoError(t, err)
	require.EqualValues(t, 9, decision.PolicyVersion)
}

func TestRuntimeOwnsGrantConditions(t *testing.T) {
	dataset := assessmentDataset(t)
	runtime, err := authzruntime.NewRuntime(context.Background(), &mutableSource{dataset: dataset}, authorization.NewEvaluator(), authzruntime.WithAttributeProviders(authzfixture.Policy()))
	require.NoError(t, err)
	request := checkRequest(t, 2, "retry", "plan")
	before, err := runtime.Check(context.Background(), request)
	require.NoError(t, err)
	require.False(t, before.Allowed)
	*dataset.Grants[1].Constraints.AllOf[0].Value.String = "plan"
	dataset.Grants[1].Constraints = constraint.Empty()
	after, err := runtime.Check(context.Background(), request)
	require.NoError(t, err)
	require.False(t, after.Allowed, "mutating the source must not change published policy")
}

// durableSource exposes the independent read-only version port.
type durableSource struct {
	dataset  authzruntime.Dataset
	versions map[string]int64
	load     func(context.Context) error
	read     func(context.Context) error
}

func (s *durableSource) Load(ctx context.Context) (authzruntime.Dataset, error) {
	if s.load != nil {
		if err := s.load(ctx); err != nil {
			return authzruntime.Dataset{}, err
		}
	}
	return s.dataset, nil
}
func (s *durableSource) ReadVersions(ctx context.Context) (map[string]int64, error) {
	if s.read != nil {
		if err := s.read(ctx); err != nil {
			return nil, err
		}
	}
	return s.versions, nil
}
func TestFreshnessBoundaryAndLostEventRecovery(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	source := &durableSource{dataset: assessmentDataset(t), versions: map[string]int64{"fangcun": 9}}
	runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(), authzruntime.WithAttributeProviders(authzfixture.Policy()), authzruntime.WithClock(func() time.Time { return now }))
	require.NoError(t, err)
	request := checkRequest(t, 2, "retry", "adhoc")
	for _, delta := range []time.Duration{59 * time.Second, time.Second} {
		now = now.Add(delta)
		runtime.RecordPolicyVersionEvent("fangcun", 8, now) // duplicates are not proof
		_, err = runtime.Check(context.Background(), request)
		_, snapshotErr := runtime.GetAuthorizationSnapshot(context.Background(), request.Subject, "fangcun", "")
		_, rolesErr := runtime.EffectiveRoleNamesForSubject(context.Background(), request.Subject, "fangcun")
		for _, got := range []error{err, snapshotErr, rolesErr} {
			if delta == time.Second {
				require.True(t, perrors.IsCode(got, code.ErrAuthorizationPolicyUnavailable), "%v", got)
			} else {
				require.NoError(t, got)
			}
		}
	}
	source.dataset.Versions = map[string]int64{"fangcun": 10}
	source.versions = map[string]int64{"fangcun": 10}
	source.dataset.Grants = nil // revoked, event deliberately lost
	require.NoError(t, runtime.Reconcile(context.Background()))
	decision, err := runtime.Check(context.Background(), request)
	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.EqualValues(t, 10, decision.PolicyVersion)
}
func TestSlowVersionReadAndLoadUseReadStartAsProof(t *testing.T) {
	for _, reload := range []bool{false, true} {
		t.Run(fmt.Sprint(reload), func(t *testing.T) {
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			source := &durableSource{dataset: assessmentDataset(t), versions: map[string]int64{"fangcun": 9}}
			runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(), authzruntime.WithAttributeProviders(authzfixture.Policy()), authzruntime.WithClock(func() time.Time { return now }))
			require.NoError(t, err)
			slow := func(context.Context) error { now = now.Add(59 * time.Second); return nil }
			if reload {
				source.load = slow
				require.NoError(t, runtime.LoadPolicy(context.Background()))
			} else {
				source.read = slow
				require.NoError(t, runtime.Reconcile(context.Background()))
			}
			now = now.Add(time.Second)
			_, err = runtime.Check(context.Background(), checkRequest(t, 2, "retry", "adhoc"))
			require.True(t, perrors.IsCode(err, code.ErrAuthorizationPolicyUnavailable))
		})
	}
}
func TestConcurrentReloadIsCancellableAndChecksDoNotBlock(t *testing.T) {
	source := &durableSource{dataset: assessmentDataset(t)}
	runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(), authzruntime.WithAttributeProviders(authzfixture.Policy()))
	require.NoError(t, err)
	entered := make(chan struct{})
	release := make(chan struct{})
	var loads atomic.Int32
	source.load = func(ctx context.Context) error {
		loads.Add(1)
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	done := make(chan error, 1)
	go func() { done <- runtime.LoadPolicy(context.Background()) }()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, runtime.LoadPolicy(ctx), context.Canceled)
	var wg sync.WaitGroup
	request := checkRequest(t, 2, "retry", "adhoc")
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, err := runtime.Check(context.Background(), request)
				if err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, loads.Load())
	close(release)
	require.NoError(t, <-done)
}
func TestObservedTargetAndReadFailureNeverRenewProof(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	source := &durableSource{dataset: assessmentDataset(t), versions: map[string]int64{"fangcun": 9}}
	runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(), authzruntime.WithAttributeProviders(authzfixture.Policy()), authzruntime.WithClock(func() time.Time { return now }))
	require.NoError(t, err)
	now = now.Add(30 * time.Second)
	runtime.RecordPolicyVersionEvent("fangcun", 10, now)
	require.NoError(t, runtime.LoadPolicy(context.Background())) // v9 cannot satisfy known v10
	require.Error(t, runtime.Reconcile(context.Background()))
	source.read = func(context.Context) error { return fmt.Errorf("database offline") }
	now = now.Add(30 * time.Second)
	require.Error(t, runtime.Reconcile(context.Background()))
	_, err = runtime.Check(context.Background(), checkRequest(t, 2, "retry", "adhoc"))
	require.True(t, perrors.IsCode(err, code.ErrAuthorizationPolicyUnavailable))
}

func TestConditionalPolicyRequiresConfiguredCoverage(t *testing.T) {
	source := &mutableSource{dataset: assessmentDataset(t)}
	_, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator())
	require.ErrorContains(t, err, "requires trusted attribute providers")
}

func TestSnapshotOutputOwnershipAndConcurrentPublication(t *testing.T) {
	source := &synchronizedSource{data: assessmentDataset(t)}
	runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator(), authzruntime.WithAttributeProviders(authzfixture.Policy()))
	require.NoError(t, err)
	request := checkRequest(t, 2, "retry", "adhoc")
	snapshot, err := runtime.GetAuthorizationSnapshot(context.Background(), request.Subject, "fangcun", "qs")
	require.NoError(t, err)
	snapshot.EffectiveRoles[0] = "corrupted"
	snapshot.Permissions[0].Resource = "corrupted"
	source.data.Resources[0].AttributeSchema.Attributes[0].AllowedStringValues[0] = "corrupted"
	source.data.Resources[0].Actions = nil
	decision, err := runtime.Check(context.Background(), request)
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	source.data = assessmentDataset(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				decision, err := runtime.Check(context.Background(), request)
				if err != nil || !decision.Allowed {
					t.Errorf("Check: %+v, %v", decision, err)
				}
			}
		}()
	}
	for i := 0; i < 20; i++ {
		require.NoError(t, runtime.LoadPolicy(context.Background()))
	}
	wg.Wait()
	fresh, err := runtime.GetAuthorizationSnapshot(context.Background(), request.Subject, "fangcun", "qs")
	require.NoError(t, err)
	require.NotContains(t, fresh.EffectiveRoles, "corrupted")
}
