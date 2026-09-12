package authz

import (
	"context"
	policyDomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"
	"net"
	"sort"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authzv4 "github.com/FangcunMount/iam/v5/api/grpc/iam/authz/v4"
	assignmentApp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignment"
	assignmentadmission "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/assignmentadmission"
	authzapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/subject"
	authzruntime "github.com/FangcunMount/iam/v5/internal/apiserver/infra/authz/runtime"
	authzfixture "github.com/FangcunMount/iam/v5/internal/apiserver/testfixtures/assessment"
	"github.com/FangcunMount/iam/v5/internal/pkg/code"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const assessmentResource = authzfixture.Resource

func TestAuthorizationServerRequiresServiceIdentity(t *testing.T) {
	srv := &authorizationServer{
		checker: &checkerFake{},
	}
	_, err := srv.Check(context.Background(), &authzv4.CheckRequest{
		Subject: "user:1", Resource: assessmentResource, Action: "retry",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestAuthorizationServerSnapshotPreservesAuthorizationMode(t *testing.T) {
	reader := &snapshotReaderFake{snapshot: authzapp.SubjectSnapshot{
		DirectRoles:    []string{"qs:assessment_operator"},
		EffectiveRoles: []string{"qs:assessment_operator", "stale-inherited"}, PolicyVersion: 7,
		Permissions: []authzapp.PermissionEntry{{
			Resource: assessmentResource, Action: "retry", Mode: authzapp.ModeUnconditional,
		}},
	}}
	srv := &authorizationServer{snapshotReader: reader}
	resp, err := srv.GetAuthorizationSnapshot(serviceContext("qs-apiserver.svc"), &authzv4.GetAuthorizationSnapshotRequest{
		Subject: "user:1", AppName: "qs",
	})
	require.NoError(t, err)
	require.EqualValues(t, 7, resp.PolicyVersion)
	require.Equal(t, []string{"qs:assessment_operator"}, resp.Roles)
	require.Equal(t, []string{"qs:assessment_operator"}, resp.DirectRoles)
	require.Equal(t, authzv4.AuthorizationMode_UNCONDITIONAL, resp.Permissions[0].Mode)
}

func TestAuthorizationServerAssignmentsUseV3AndConstraints(t *testing.T) {
	policy, err := assignmentadmission.New(assignmentadmission.Config{
		DefaultPolicy: "deny",
		Services: map[string]assignmentadmission.ServiceConstraint{
			"qs-apiserver.svc": {
				SubjectTypes: []string{"user"},
				Roles:        []string{"qs:evaluator", "qs:staff"}, RequireDelegatedActorOnGrant: true,
			},
		},
	})
	require.NoError(t, err)
	commands := &assignmentCommandsFake{}
	srv := &authorizationServer{assignments: commands, assignmentAdmission: policy}

	grantResponse, err := srv.GrantAssignment(serviceContext("qs-apiserver.svc"), &authzv4.GrantAssignmentRequest{
		Subject: "user:100", RoleName: "qs:evaluator", GrantedBy: "user:1",
	})
	require.NoError(t, err)
	require.Len(t, commands.grants, 1)
	require.EqualValues(t, 11, grantResponse.PolicyVersion)

	revokeResponse, err := srv.RevokeAssignment(serviceContext("qs-apiserver.svc"), &authzv4.RevokeAssignmentRequest{
		Subject: "user:100", RoleName: "qs:evaluator",
	})
	require.NoError(t, err)
	require.Equal(t, "service:qs-apiserver.svc", commands.revokes[0].ChangedBy)
	require.EqualValues(t, 12, revokeResponse.PolicyVersion)

	replaceResponse, err := srv.ReplaceManagedAssignments(serviceContext("qs-apiserver.svc"), &authzv4.ReplaceManagedAssignmentsRequest{
		Subject: "user:100", RoleNames: []string{"qs:evaluator", "qs:staff"},
		ChangedBy: "user:1", Reason: "staff role update",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"qs:evaluator", "qs:staff"}, replaceResponse.DirectRoles)
	require.EqualValues(t, 13, replaceResponse.PolicyVersion)
	require.True(t, replaceResponse.Changed)
	require.Len(t, commands.replacements, 1)
}

func TestAuthorizationServerRejectsNilAssignmentAdmissionPolicy(t *testing.T) {
	srv := &authorizationServer{assignments: &assignmentCommandsFake{}}
	ctx := serviceContext("qs-apiserver.svc")

	_, err := srv.GrantAssignment(ctx, &authzv4.GrantAssignmentRequest{
		Subject: "user:100", RoleName: "qs:evaluator", GrantedBy: "user:1",
	})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = srv.RevokeAssignment(ctx, &authzv4.RevokeAssignmentRequest{
		Subject: "user:100", RoleName: "qs:evaluator",
	})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = srv.ReplaceManagedAssignments(ctx, &authzv4.ReplaceManagedAssignmentsRequest{
		Subject: "user:100", RoleNames: []string{"qs:evaluator"}, ChangedBy: "user:1",
	})
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestAuthorizationV3GRPCAssessmentRetryMatrix(t *testing.T) {
	checker := newMatrixRuntime(t)
	client, cleanup := newAuthorizationTestClient(t, checker)
	t.Cleanup(cleanup)

	tests := []struct {
		name       string
		subject    string
		originType string
		allowed    bool
	}{
		{name: "admin adhoc", subject: "user:1", originType: "adhoc", allowed: true},
		{name: "admin plan", subject: "user:1", originType: "plan", allowed: true},
		{name: "evaluator adhoc", subject: "user:2", originType: "adhoc", allowed: true},
		{name: "evaluator plan", subject: "user:2", originType: "plan", allowed: true},
		{name: "plan manager adhoc", subject: "user:3", originType: "adhoc", allowed: true},
		{name: "plan manager plan", subject: "user:3", originType: "plan", allowed: true},
		{name: "other", subject: "user:4", originType: "adhoc", allowed: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := client.Check(context.Background(), assessmentCheckRequest(tt.subject, tt.originType))
			require.NoError(t, err)
			require.Equal(t, tt.allowed, response.GetAllowed())
			require.EqualValues(t, 41, response.GetPolicyVersion())
			if tt.allowed {
				require.Equal(t, authzv4.DecisionReason_ALLOWED, response.GetReason())
				require.NotEmpty(t, response.GetMatchedGrantId())
				require.NotEmpty(t, response.GetMatchedRole())
			} else {
				require.Equal(t, authzv4.DecisionReason_NOT_MATCHED, response.GetReason())
				require.Equal(t, authorization.DenyCodePolicyNotMatched, response.GetDenyCode())
			}
		})
	}
}

func TestAuthorizationV3GRPCLatencyBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("latency acceptance is disabled in short mode")
	}
	client, cleanup := newAuthorizationTestClient(t, newMatrixRuntime(t))
	t.Cleanup(cleanup)
	request := assessmentCheckRequest("user:2", "adhoc")
	for i := 0; i < 100; i++ {
		_, err := client.Check(context.Background(), request)
		require.NoError(t, err)
	}

	const samples = 2000
	latencies := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		started := time.Now()
		_, err := client.Check(context.Background(), request)
		require.NoError(t, err)
		latencies = append(latencies, time.Since(started))
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	p95 := latencies[(samples*95+99)/100-1]
	p99 := latencies[(samples*99+99)/100-1]
	t.Logf("AuthZ v3 in-process gRPC latency: samples=%d p95=%s p99=%s", samples, p95, p99)
	require.LessOrEqual(t, p95, 20*time.Millisecond)
	require.LessOrEqual(t, p99, 50*time.Millisecond)
}

func assessmentCheckRequest(subjectKey, originType string) *authzv4.CheckRequest {
	return &authzv4.CheckRequest{Subject: subjectKey, Resource: "qs:evaluation:collection:assessments", Action: "retry"}
}

func newMatrixRuntime(t *testing.T) *authzruntime.Runtime {
	t.Helper()
	assessment, err := resource.NewResource(
		assessmentResource,
		[]string{"retry", "force_retry", "batch_evaluate"},
		resource.WithID(resource.NewResourceID(20)),
		resource.WithDisplayName("Assessments"),
	)
	require.NoError(t, err)
	admin, err := permissiongrant.NewSystem(
		meta.FromUint64(11), resource.ResourceID{}, "qs:*:*:*", "*", "contract-test",
	)
	require.NoError(t, err)
	evaluator, err := permissiongrant.New(
		meta.FromUint64(12), assessment.ID, assessment.KeyString(), "retry", "contract-test",
	)
	require.NoError(t, err)
	planManager, err := permissiongrant.New(
		meta.FromUint64(13), assessment.ID, assessment.KeyString(), "retry", "contract-test",
	)
	require.NoError(t, err)
	admin.ID, evaluator.ID, planManager.ID = meta.FromUint64(100), meta.FromUint64(102), meta.FromUint64(103)

	runtime, err := authzruntime.NewRuntime(context.Background(), staticRuntimeSource{dataset: authzruntime.Dataset{
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
		Grants:    []*permissiongrant.Grant{&admin, &evaluator, &planManager},
		Resources: []*resource.Resource{&assessment},
		Version:   41,
	}}, authorization.NewEvaluator())
	require.NoError(t, err)
	return runtime
}

func newAuthorizationTestClient(t *testing.T, checker authorizationChecker) (authzv4.AuthorizationServiceClient, func()) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(
		ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (any, error) {
		return handler(interceptors.ContextWithServiceIdentity(ctx, &interceptors.ServiceIdentity{
			ServiceName: authzfixture.Service,
		}), req)
	}))
	authzv4.RegisterAuthorizationServiceServer(server, &authorizationServer{
		checker: checker,
	})
	go func() { _ = server.Serve(listener) }()
	connection, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
	)
	require.NoError(t, err)
	cleanup := func() {
		_ = connection.Close()
		server.Stop()
		_ = listener.Close()
	}
	return authzv4.NewAuthorizationServiceClient(connection), cleanup
}

type checkerFake struct {
	decision authorization.Decision
	err      error
	calls    []authorization.Request
}

type snapshotReaderFake struct {
	snapshot authzapp.SubjectSnapshot
	err      error
}

func (f *snapshotReaderFake) Read(_ context.Context, _ subject.Ref, _ string) (authzapp.SubjectSnapshot, error) {
	if f.err != nil {
		return authzapp.SubjectSnapshot{}, f.err
	}
	return f.snapshot, nil
}

type assignmentCommandsFake struct {
	grants       []assignmentApp.GrantByRoleNameCommand
	revokes      []assignmentApp.RevokeByRoleNameCommand
	replacements []assignmentApp.ReplaceManagedAssignmentsCommand
	grantErr     error
	revokeErr    error
	replaceErr   error
}

func (f *assignmentCommandsFake) GrantByRoleName(_ context.Context, cmd assignmentApp.GrantByRoleNameCommand) (int64, error) {
	f.grants = append(f.grants, cmd)
	return 11, f.grantErr
}

func (f *assignmentCommandsFake) RevokeByRoleName(_ context.Context, cmd assignmentApp.RevokeByRoleNameCommand) (int64, error) {
	f.revokes = append(f.revokes, cmd)
	return 12, f.revokeErr
}

func (f *assignmentCommandsFake) ReplaceManagedAssignments(_ context.Context, cmd assignmentApp.ReplaceManagedAssignmentsCommand) (assignmentApp.ReplaceManagedAssignmentsResult, error) {
	f.replacements = append(f.replacements, cmd)
	return assignmentApp.ReplaceManagedAssignmentsResult{DirectRoles: cmd.RoleNames, PolicyVersion: 13, Changed: true}, f.replaceErr
}

func serviceContext(serviceName string) context.Context {
	return interceptors.ContextWithServiceIdentity(context.Background(), &interceptors.ServiceIdentity{ServiceName: serviceName})
}

func TestAuthorizationUnavailableRemainsGRPCUnavailable(t *testing.T) {
	client, closeClient := newAuthorizationTestClient(t, &checkerFake{err: perrors.WithCode(code.ErrAuthorizationPolicyUnavailable, "expired")})
	defer closeClient()
	_, err := client.Check(context.Background(), assessmentCheckRequest("user:2", "adhoc"))
	require.Equal(t, codes.Unavailable, status.Code(err))
}

type staticRuntimeSource struct{ dataset authzruntime.Dataset }

func (s staticRuntimeSource) Load(context.Context) (authzruntime.Dataset, error) {
	return s.dataset, nil
}

func TestRetiredObjectContextRejectedBeforeChecking(t *testing.T) {
	checker := &checkerFake{decision: authorization.Decision{Allowed: true}}
	server := &authorizationServer{checker: checker}
	for _, object := range []*authzv4.ObjectContext{{ObjectId: "1"}, {Attributes: []*authzv4.ObjectAttribute{{Key: "object.origin_type"}}}} {
		_, err := server.Check(serviceContext("qs-apiserver.svc"), &authzv4.CheckRequest{Subject: "user:1", Resource: assessmentResource, Action: "retry", ObjectContext: object})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	require.Empty(t, checker.calls)
}
func (f *checkerFake) Check(_ context.Context, r authorization.Request) (authorization.Decision, error) {
	f.calls = append(f.calls, r)
	return f.decision, f.err
}

func TestAssignmentFactsRequireManagedCallerAndExplicitOptIn(t *testing.T) {
	policy, err := assignmentadmission.New(assignmentadmission.Config{DefaultPolicy: "deny", Services: map[string]assignmentadmission.ServiceConstraint{"qs-apiserver.svc": {SubjectTypes: []string{"user"}, Roles: []string{"qs:result_reviewer"}, RequireDelegatedActorOnGrant: true}}})
	require.NoError(t, err)
	reader := &snapshotReaderFake{snapshot: authzapp.SubjectSnapshot{PolicyVersion: 7, AssignmentFactsComplete: true, AssignmentFacts: []authzapp.AssignmentRoleFact{{RoleID: "9", RoleName: "platform_admin", ManagementProtection: "protected"}}}}
	value, scopeErr := scope.New(1, scope.Stores, []meta.ID{123456789012345678})
	require.NoError(t, scopeErr)
	reader.snapshot.AssignmentScopes = []authzapp.AssignmentScopeFact{{AssignmentID: "100", Role: reader.snapshot.AssignmentFacts[0], Scope: &value}, {AssignmentID: "101", Role: reader.snapshot.AssignmentFacts[0]}}
	server := &authorizationServer{snapshotReader: reader, assignmentAdmission: policy}
	request := &authzv4.GetAuthorizationSnapshotRequest{Subject: "user:1", AppName: "qs"}
	response, err := server.GetAuthorizationSnapshot(serviceContext("qs-apiserver.svc"), request)
	require.NoError(t, err)
	require.False(t, response.AssignmentFactsComplete)
	require.Empty(t, response.AssignmentFacts)
	require.Empty(t, response.AssignmentScopes)
	request.IncludeAssignmentFacts = true
	_, err = server.GetAuthorizationSnapshot(serviceContext("untrusted.svc"), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	response, err = server.GetAuthorizationSnapshot(serviceContext("qs-apiserver.svc"), request)
	require.NoError(t, err)
	require.True(t, response.AssignmentFactsComplete)
	require.Equal(t, "platform_admin", response.AssignmentFacts[0].RoleName)
	require.Len(t, response.AssignmentScopes, 2)
	require.Equal(t, "100", response.AssignmentScopes[0].AssignmentId)
	require.Equal(t, []string{"123456789012345678"}, response.AssignmentScopes[0].Scope.StoreIds)
	require.Nil(t, response.AssignmentScopes[1].Scope)
	reader.snapshot.AssignmentFactsComplete = false
	_, err = server.GetAuthorizationSnapshot(serviceContext("qs-apiserver.svc"), request)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSnapshotTransportsScopeWithLosslessIDs(t *testing.T) {
	value, err := scope.New(1, scope.Stores, []meta.ID{123456789012345678})
	require.NoError(t, err)
	reader := &snapshotReaderFake{snapshot: authzapp.SubjectSnapshot{PolicyVersion: 7, Permissions: []authzapp.PermissionEntry{{Resource: assessmentResource, Action: "retry", Mode: authzapp.ModeUnconditional, Scopes: []scope.Scope{value}}}}}
	server := &authorizationServer{snapshotReader: reader}
	response, err := server.GetAuthorizationSnapshot(serviceContext("qs-apiserver.svc"), &authzv4.GetAuthorizationSnapshotRequest{Subject: "user:1", AppName: "qs"})
	require.NoError(t, err)
	require.EqualValues(t, 1, response.ScopeContractVersion)
	require.Len(t, response.Permissions, 1)
	require.Len(t, response.Permissions[0].Scopes, 1)
	got := response.Permissions[0].Scopes[0]
	require.Equal(t, "1", got.OrgId)
	require.Equal(t, []string{"123456789012345678"}, got.StoreIds)
	require.Equal(t, authzv4.DataScopeKind_STORES, got.Kind)
}

func TestScopedAssignmentRPCPreservesAdmissionAndCompany(t *testing.T) {
	policy, err := assignmentadmission.New(assignmentadmission.Config{DefaultPolicy: "deny", Services: map[string]assignmentadmission.ServiceConstraint{"qs-apiserver.svc": {SubjectTypes: []string{"user"}, Roles: []string{"qs:assessment_operator"}, RequireDelegatedActorOnGrant: true}}})
	require.NoError(t, err)
	commands := &assignmentCommandsFake{}
	server := &authorizationServer{assignments: commands, assignmentAdmission: policy}
	request := &authzv4.ReplaceScopedAssignmentsRequest{ExpectedPolicyVersion: 7, Subject: "user:100", OrgId: "1", ChangedBy: "user:1", Roles: []*authzv4.ScopedRoleAssignment{{RoleName: "qs:assessment_operator", Scope: &authzv4.DataScope{OrgId: "1", Kind: authzv4.DataScopeKind_STORES, StoreIds: []string{"10"}}}}}
	_, err = server.ReplaceScopedAssignments(serviceContext("qs-apiserver.svc"), request)
	require.NoError(t, err)
	require.Len(t, commands.replacements, 1)
	cmd := commands.replacements[0]
	require.EqualValues(t, 1, cmd.OrgID)
	require.EqualValues(t, 7, cmd.ExpectedPolicyVersion)
	require.Empty(t, cmd.RoleNames)
	require.True(t, cmd.ScopedRoles[0].Scope.ContainsStore(1, 10))
	_, err = server.ReplaceScopedAssignments(serviceContext("untrusted.svc"), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	request.Roles[0].Scope.OrgId = "2"
	_, err = server.ReplaceScopedAssignments(serviceContext("qs-apiserver.svc"), request)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	request.Roles[0].Scope.OrgId = "1"
	request.Roles[0].RoleName = "platform_admin"
	_, err = server.ReplaceScopedAssignments(serviceContext("qs-apiserver.svc"), request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Len(t, commands.replacements, 1, "rejected calls must not enter writes")
	request.Roles[0].RoleName = "qs:assessment_operator"
	request.ExpectedPolicyVersion = 0
	_, err = server.ReplaceScopedAssignments(serviceContext("qs-apiserver.svc"), request)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Len(t, commands.replacements, 1)
	request.ExpectedPolicyVersion = 7
	commands.replaceErr = policyDomain.ErrStaleVersion
	_, err = server.ReplaceScopedAssignments(serviceContext("qs-apiserver.svc"), request)
	require.Equal(t, codes.Aborted, status.Code(err))

}
