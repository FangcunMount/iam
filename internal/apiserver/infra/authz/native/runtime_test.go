package native_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/runtime"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/native"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
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
		{name: "evaluator plan", userID: 2, originType: "plan", allowed: false},
		{name: "plan manager adhoc", userID: 3, originType: "adhoc", allowed: false},
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

func TestRuntimeMissingAttributeDeniesWithReason(t *testing.T) {
	runtime := newAssessmentRuntime(t)
	sub, err := subject.NewUserRef(meta.FromUint64(2))
	require.NoError(t, err)
	object, err := authzruntime.NewObjectContext("assessment-1", nil)
	require.NoError(t, err)
	request, err := authzruntime.NewRequest(sub, "fangcun", assessmentResource, "retry", object)
	require.NoError(t, err)

	decision, err := runtime.Check(context.Background(), request)
	require.NoError(t, err)
	require.False(t, decision.Allowed)
	require.Equal(t, authzruntime.ReasonAttributeMissing, decision.Reason)
	require.Equal(t, []string{attribute.ObjectOriginType}, decision.MissingAttributeKeys)
}

func TestRuntimeSnapshotPreservesConditionalMode(t *testing.T) {
	runtime := newAssessmentRuntime(t)
	sub, err := subject.NewUserRef(meta.FromUint64(2))
	require.NoError(t, err)

	snapshot, err := runtime.GetAuthorizationSnapshot(context.Background(), sub, "fangcun", "qs")
	require.NoError(t, err)
	require.Contains(t, snapshot.Roles, "qs:evaluator")
	require.Contains(t, snapshot.Permissions, authzruntime.PermissionEntry{
		Resource: assessmentResource, Action: "retry", Mode: authzruntime.ModeObjectCheckRequired,
	})
	require.Contains(t, snapshot.Permissions, authzruntime.PermissionEntry{
		Resource: assessmentResource, Action: "batch_evaluate", Mode: authzruntime.ModeUnconditional,
	})

	allowed, err := runtime.AuthorizeRoute(context.Background(), sub.String(), "fangcun", assessmentResource, "retry")
	require.NoError(t, err)
	require.False(t, allowed, "generic route authorization must not accept conditional grants")
	allowed, err = runtime.AuthorizeRoute(context.Background(), sub.String(), "fangcun", assessmentResource, "batch_evaluate")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestRuntimeSnapshotIncludesQSAdminWildcardAsUnconditionalCandidate(t *testing.T) {
	runtime := newAssessmentRuntime(t)
	sub, err := subject.NewUserRef(meta.FromUint64(1))
	require.NoError(t, err)

	snapshot, err := runtime.GetAuthorizationSnapshot(context.Background(), sub, "fangcun", "qs")
	require.NoError(t, err)
	require.Contains(t, snapshot.Roles, "qs:admin")
	require.Contains(t, snapshot.Permissions, authzruntime.PermissionEntry{
		Resource: "qs:*:*:*", Action: "*", Mode: authzruntime.ModeUnconditional,
	})
}

func TestRuntimeFailedReloadKeepsPreviousSnapshot(t *testing.T) {
	source := &mutableSource{dataset: assessmentDataset(t)}
	runtime, err := native.NewRuntime(context.Background(), source)
	require.NoError(t, err)

	source.mu.Lock()
	source.err = errors.New("database unavailable")
	source.mu.Unlock()
	require.Error(t, runtime.LoadPolicy(context.Background()))

	decision, err := runtime.Check(context.Background(), checkRequest(t, 2, "retry", "adhoc"))
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	ready, reloadErr, _ := runtime.ReloadHealth()
	require.False(t, ready)
	require.ErrorContains(t, reloadErr, "database unavailable")
}

func TestBuildSnapshotRejectsInheritanceCycle(t *testing.T) {
	dataset := assessmentDataset(t)
	dataset.Inheritances = []native.InheritanceRecord{
		{TenantID: "fangcun", RoleID: meta.FromUint64(12), InheritedRoleID: meta.FromUint64(13)},
		{TenantID: "fangcun", RoleID: meta.FromUint64(13), InheritedRoleID: meta.FromUint64(12)},
	}
	_, err := native.BuildSnapshot(dataset, time.Time{})
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

type mutableSource struct {
	mu      sync.Mutex
	dataset native.Dataset
	err     error
}

func (s *mutableSource) Load(context.Context) (native.Dataset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dataset, s.err
}

func newAssessmentRuntime(t testing.TB) *native.Runtime {
	t.Helper()
	runtime, err := native.NewRuntime(context.Background(), &mutableSource{dataset: assessmentDataset(t)})
	require.NoError(t, err)
	return runtime
}

func assessmentDataset(t testing.TB) native.Dataset {
	t.Helper()
	assessment, err := resource.NewResource(
		assessmentResource,
		[]string{"retry", "force_retry", "batch_evaluate"},
		resource.WithID(resource.NewResourceID(20)),
		resource.WithAttributeSchema(attribute.AssessmentSchema()),
	)
	require.NoError(t, err)
	adhoc, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")))
	require.NoError(t, err)
	plan, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("plan")))
	require.NoError(t, err)
	admin, err := permissiongrant.NewSystem(meta.FromUint64(11), "fangcun", resource.ResourceID{}, "qs:*:*:*", "*", constraint.Empty(), "bootstrap")
	require.NoError(t, err)
	evaluatorRetry, err := permissiongrant.New(meta.FromUint64(12), "fangcun", assessment.ID, assessment.KeyString(), "retry", adhoc, "bootstrap")
	require.NoError(t, err)
	evaluatorBatch, err := permissiongrant.New(meta.FromUint64(12), "fangcun", assessment.ID, assessment.KeyString(), "batch_evaluate", constraint.Empty(), "bootstrap")
	require.NoError(t, err)
	planRetry, err := permissiongrant.New(meta.FromUint64(13), "fangcun", assessment.ID, assessment.KeyString(), "retry", plan, "bootstrap")
	require.NoError(t, err)
	admin.ID = meta.FromUint64(101)
	evaluatorRetry.ID = meta.FromUint64(102)
	evaluatorBatch.ID = meta.FromUint64(103)
	planRetry.ID = meta.FromUint64(104)

	return native.Dataset{
		Roles: []native.RoleRecord{
			{ID: meta.FromUint64(11), TenantID: "fangcun", Name: "qs:admin"},
			{ID: meta.FromUint64(12), TenantID: "fangcun", Name: "qs:evaluator"},
			{ID: meta.FromUint64(13), TenantID: "fangcun", Name: "qs:evaluation_plan_manager"},
			{ID: meta.FromUint64(14), TenantID: "fangcun", Name: "qs:staff"},
		},
		Assignments: []native.AssignmentRecord{
			{TenantID: "fangcun", SubjectKey: "user:1", RoleID: meta.FromUint64(11)},
			{TenantID: "fangcun", SubjectKey: "user:2", RoleID: meta.FromUint64(12)},
			{TenantID: "fangcun", SubjectKey: "user:3", RoleID: meta.FromUint64(13)},
			{TenantID: "fangcun", SubjectKey: "user:4", RoleID: meta.FromUint64(14)},
		},
		Grants:    []*permissiongrant.Grant{&admin, &evaluatorRetry, &evaluatorBatch, &planRetry},
		Resources: []*resource.Resource{&assessment},
		Versions:  map[string]int64{"fangcun": 9},
	}
}

func checkRequest(t testing.TB, userID uint64, action, originType string) authzruntime.Request {
	t.Helper()
	sub, err := subject.NewUserRef(meta.FromUint64(userID))
	require.NoError(t, err)
	object, err := authzruntime.NewObjectContext("assessment-1", constraint.Attributes{
		attribute.ObjectOriginType: constraint.StringValue(originType),
	})
	require.NoError(t, err)
	request, err := authzruntime.NewRequest(sub, "fangcun", assessmentResource, action, object)
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
