package runtime_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	authorizationapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

const assessmentResource = "qs:evaluation:collection:assessments"

func TestRuntimeAssessmentRetryMatrix(t *testing.T) {
	runtime := newAssessmentRuntime(t)
	cases := []struct {
		name       string
		userID     uint64
		originType string
		allowed    bool
	}{
		{name: "admin adhoc", userID: 1, originType: "adhoc", allowed: true},
		{name: "admin plan", userID: 1, originType: "plan", allowed: true},
		{name: "evaluator adhoc", userID: 2, originType: "adhoc", allowed: true},
		{name: "evaluator plan", userID: 2, originType: "plan", allowed: true},
		{name: "plan manager adhoc", userID: 3, originType: "adhoc", allowed: true},
		{name: "plan manager plan", userID: 3, originType: "plan", allowed: true},
		{name: "other", userID: 4, originType: "adhoc", allowed: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := runtime.Check(context.Background(), checkRequest(t, tc.userID, "retry", tc.originType))
			require.NoError(t, err)
			require.Equal(t, tc.allowed, decision.Allowed)
			require.EqualValues(t, 9, decision.PolicyVersion)
		})
	}
}

func TestRuntimeSnapshotContainsOnlyUnconditionalPermissions(t *testing.T) {
	runtime := newAssessmentRuntime(t)
	sub, err := subject.NewUserRef(meta.FromUint64(2))
	require.NoError(t, err)

	snapshot, err := runtime.GetAuthorizationSnapshot(context.Background(), sub, "qs")
	require.NoError(t, err)
	require.Equal(t, []string{"qs:evaluator"}, snapshot.DirectRoles)
	require.Equal(t, []string{"qs:evaluator"}, snapshot.EffectiveRoles)
	require.Contains(t, snapshot.Permissions, authorizationapp.PermissionEntry{
		Resource: assessmentResource, Action: "retry", Mode: authorizationapp.ModeUnconditional,
	})
	require.Contains(t, snapshot.Permissions, authorizationapp.PermissionEntry{
		Resource: assessmentResource, Action: "batch_evaluate", Mode: authorizationapp.ModeUnconditional,
	})

	routeDecisions := authorizationapp.NewRouteDecisionService(authorizationapp.NewDecisionService(runtime))
	allowed, err := routeDecisions.CheckRoutePermission(context.Background(), sub.String(), assessmentResource, "retry")
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = routeDecisions.CheckRoutePermission(context.Background(), sub.String(), assessmentResource, "batch_evaluate")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestRuntimeSnapshotIncludesQSAdminWildcardAsUnconditionalCandidate(t *testing.T) {
	runtime := newAssessmentRuntime(t)
	sub, err := subject.NewUserRef(meta.FromUint64(1))
	require.NoError(t, err)

	snapshot, err := runtime.GetAuthorizationSnapshot(context.Background(), sub, "qs")
	require.NoError(t, err)
	require.Contains(t, snapshot.DirectRoles, "qs:admin")
	require.Contains(t, snapshot.EffectiveRoles, "qs:admin")
	require.Contains(t, snapshot.Permissions, authorizationapp.PermissionEntry{
		Resource: "qs:*:*:*", Action: "*", Mode: authorizationapp.ModeUnconditional,
	})
}

func TestRuntimeFailedReloadKeepsPreviousSnapshot(t *testing.T) {
	source := &mutableSource{dataset: assessmentDataset(t)}
	runtime, err := authzruntime.NewRuntime(context.Background(), source, authorization.NewEvaluator())
	require.NoError(t, err)

	source.mu.Lock()
	source.err = errors.New("database unavailable")
	source.mu.Unlock()
	require.Error(t, runtime.LoadPolicy(context.Background()))

	decision, err := runtime.Check(context.Background(), checkRequest(t, 2, "retry", "adhoc"))
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	ready, reloadErr, _ := runtime.ReloadHealth()
	require.True(t, ready, "transient reload failure is within freshness budget")
	require.ErrorContains(t, reloadErr, "database unavailable")
}

type mutableSource struct {
	mu      sync.Mutex
	dataset authzruntime.Dataset
	err     error
}

func (s *mutableSource) Load(context.Context) (authzruntime.Dataset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dataset, s.err
}

func newAssessmentRuntime(t testing.TB) *authzruntime.Runtime {
	t.Helper()
	runtime, err := authzruntime.NewRuntime(
		context.Background(),
		&mutableSource{dataset: assessmentDataset(t)},
		authorization.NewEvaluator(),
	)
	require.NoError(t, err)
	return runtime
}

func assessmentDataset(t testing.TB) authzruntime.Dataset {
	t.Helper()
	assessment, err := resource.NewResource(
		assessmentResource,
		[]string{"retry", "force_retry", "batch_evaluate"},
		resource.WithID(resource.NewResourceID(20)),
		resource.WithDisplayName("Assessments"),
	)
	require.NoError(t, err)
	admin, err := permissiongrant.NewSystem(meta.FromUint64(11), resource.ResourceID{}, "qs:*:*:*", "*", "bootstrap")
	require.NoError(t, err)
	evaluatorRetry, err := permissiongrant.New(meta.FromUint64(12), assessment.ID, assessment.KeyString(), "retry", "bootstrap")
	require.NoError(t, err)
	evaluatorBatch, err := permissiongrant.New(meta.FromUint64(12), assessment.ID, assessment.KeyString(), "batch_evaluate", "bootstrap")
	require.NoError(t, err)
	planRetry, err := permissiongrant.New(meta.FromUint64(13), assessment.ID, assessment.KeyString(), "retry", "bootstrap")
	require.NoError(t, err)
	admin.ID = meta.FromUint64(101)
	evaluatorRetry.ID = meta.FromUint64(102)
	evaluatorBatch.ID = meta.FromUint64(103)
	planRetry.ID = meta.FromUint64(104)

	return authzruntime.Dataset{
		Roles: []authzruntime.RoleRecord{
			{ManagementProtection: "standard", ID: meta.FromUint64(11), Name: "qs:admin"},
			{ManagementProtection: "standard", ID: meta.FromUint64(12), Name: "qs:evaluator"},
			{ManagementProtection: "standard", ID: meta.FromUint64(13), Name: "qs:evaluation_plan_manager"},
			{ManagementProtection: "standard", ID: meta.FromUint64(14), Name: "qs:staff"},
		},
		Assignments: []authzruntime.AssignmentRecord{
			{SubjectKey: "user:1", RoleID: meta.FromUint64(11)},
			{SubjectKey: "user:2", RoleID: meta.FromUint64(12)},
			{SubjectKey: "user:3", RoleID: meta.FromUint64(13)},
			{SubjectKey: "user:4", RoleID: meta.FromUint64(14)},
		},
		Grants:    []*permissiongrant.Grant{&admin, &evaluatorRetry, &evaluatorBatch, &planRetry},
		Resources: []*resource.Resource{&assessment},
		Version:   9,
	}
}

func checkRequest(t testing.TB, userID uint64, action, originType string) authorization.Request {
	t.Helper()
	sub, err := subject.NewUserRef(meta.FromUint64(userID))
	require.NoError(t, err)
	request, err := authorization.NewRequest(sub, assessmentResource, action)
	require.NoError(t, err)
	return request
}

func BenchmarkRuntimeCheckAssessmentRetry(b *testing.B) {
	runtime := newAssessmentRuntime(b)
	request := checkRequest(b, 2, "retry", "adhoc")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decision, err := runtime.Check(ctx, request)
		if err != nil || !decision.Allowed {
			b.Fatalf("Check() decision=%+v error=%v", decision, err)
		}
	}
}

func TestAssignmentFactsPreserveGlobalAndOtherApplicationRoles(t *testing.T) {
	data := assessmentDataset(t)
	data.Roles = append(data.Roles, authzruntime.RoleRecord{ID: meta.FromUint64(99), Name: "platform_admin", ManagementProtection: "protected"}, authzruntime.RoleRecord{ID: meta.FromUint64(100), Name: "other:manager", ManagementProtection: "standard"})
	data.Assignments = append(data.Assignments, authzruntime.AssignmentRecord{SubjectKey: "user:2", RoleID: meta.FromUint64(99)}, authzruntime.AssignmentRecord{SubjectKey: "user:2", RoleID: meta.FromUint64(100)})
	snapshot, err := authzruntime.BuildSnapshot(data, time.Now())
	require.NoError(t, err)
	sub, err := subject.ParseRef("user:2")
	require.NoError(t, err)
	result, err := snapshot.SubjectSnapshot(sub, "qs")
	require.NoError(t, err)
	require.Equal(t, []string{"qs:evaluator"}, result.DirectRoles)
	require.True(t, result.AssignmentFactsComplete)
	require.Len(t, result.AssignmentFacts, 3)
	require.Contains(t, result.AssignmentFacts, authorizationapp.AssignmentRoleFact{RoleID: "99", RoleName: "platform_admin", ManagementProtection: "protected"})
}
