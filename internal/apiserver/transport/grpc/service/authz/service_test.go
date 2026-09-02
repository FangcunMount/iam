package authz

import (
	"context"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authzv3 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v3"
	assignmentApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignment"
	assignmentadmission "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignmentadmission"
	authzapp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	objectattributeadmission "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/objectattributeadmission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestAuthorizationServerRequiresServiceIdentity(t *testing.T) {
	srv := &authorizationServer{
		checker:                  &checkerFake{},
		objectAttributeAdmission: objectattributeadmission.NewDefaultPolicy(),
	}
	_, err := srv.Check(context.Background(), &authzv3.CheckRequest{
		Subject: "user:1", Domain: "fangcun", Resource: assessmentResource, Action: "retry",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestAuthorizationServerCheckMapsTypedObjectContext(t *testing.T) {
	checker := &checkerFake{decision: authorization.Decision{
		Allowed: true, Reason: authorization.ReasonAllowed,
		MatchedGrantID: meta.FromUint64(100), MatchedRole: "qs:evaluator", PolicyVersion: 12,
	}}
	srv := &authorizationServer{
		checker:                  checker,
		objectAttributeAdmission: objectattributeadmission.NewDefaultPolicy(),
	}
	ctx := serviceContext("qs-apiserver.svc")
	resp, err := srv.Check(ctx, &authzv3.CheckRequest{
		Subject: "user:1", Domain: "fangcun", Resource: assessmentResource, Action: "retry",
		ObjectContext: &authzv3.ObjectContext{
			ObjectId: "assessment-1",
			Attributes: []*authzv3.ObjectAttribute{{
				Key:   attribute.ObjectOriginType,
				Value: &authzv3.ObjectAttribute_StringValue{StringValue: "adhoc"},
			}},
		},
	})
	require.NoError(t, err)
	require.True(t, resp.Allowed)
	require.Equal(t, authzv3.DecisionReason_ALLOWED, resp.Reason)
	require.Equal(t, "100", resp.MatchedGrantId)
	require.Equal(t, "qs:evaluator", resp.MatchedRole)
	require.Len(t, checker.calls, 1)
	require.Equal(t, "assessment-1", checker.calls[0].Object.ObjectID)
	require.Equal(t, "adhoc", *checker.calls[0].Object.Attributes[attribute.ObjectOriginType].String)
}

func TestAuthorizationServerRejectsDuplicateAndUntrustedAttributes(t *testing.T) {
	srv := &authorizationServer{
		checker:                  &checkerFake{},
		objectAttributeAdmission: objectattributeadmission.NewDefaultPolicy(),
	}
	duplicate := []*authzv3.ObjectAttribute{
		{Key: attribute.ObjectOriginType, Value: &authzv3.ObjectAttribute_StringValue{StringValue: "adhoc"}},
		{Key: attribute.ObjectOriginType, Value: &authzv3.ObjectAttribute_StringValue{StringValue: "plan"}},
	}
	_, err := srv.Check(serviceContext("qs-apiserver.svc"), &authzv3.CheckRequest{
		Subject: "user:1", Domain: "fangcun", Resource: assessmentResource, Action: "retry",
		ObjectContext: &authzv3.ObjectContext{ObjectId: "assessment-1", Attributes: duplicate},
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = srv.Check(serviceContext("admin"), &authzv3.CheckRequest{
		Subject: "user:1", Domain: "fangcun", Resource: assessmentResource, Action: "retry",
		ObjectContext: &authzv3.ObjectContext{ObjectId: "assessment-1", Attributes: duplicate[:1]},
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestAuthorizationServerSnapshotPreservesAuthorizationMode(t *testing.T) {
	reader := &snapshotReaderFake{snapshot: authzapp.SubjectSnapshot{
		DirectRoles:    []string{"qs:evaluator"},
		EffectiveRoles: []string{"qs:evaluator", "qs:staff"}, PolicyVersion: 7,
		Permissions: []authzapp.PermissionEntry{{
			Resource: assessmentResource, Action: "retry", Mode: authzapp.ModeObjectCheckRequired,
		}},
	}}
	srv := &authorizationServer{snapshotReader: reader}
	resp, err := srv.GetAuthorizationSnapshot(serviceContext("qs-apiserver.svc"), &authzv3.GetAuthorizationSnapshotRequest{
		Subject: "user:1", Domain: "fangcun", AppName: "qs",
	})
	require.NoError(t, err)
	require.EqualValues(t, 7, resp.PolicyVersion)
	require.Equal(t, []string{"qs:evaluator", "qs:staff"}, resp.Roles)
	require.Equal(t, []string{"qs:evaluator"}, resp.DirectRoles)
	require.Equal(t, authzv3.AuthorizationMode_OBJECT_CHECK_REQUIRED, resp.Permissions[0].Mode)
}

func TestAuthorizationServerAssignmentsUseV3AndConstraints(t *testing.T) {
	policy, err := assignmentadmission.New(assignmentadmission.Config{
		DefaultPolicy: "deny",
		Services: map[string]assignmentadmission.ServiceConstraint{
			"qs-apiserver.svc": {
				Domains: []string{"fangcun"}, SubjectTypes: []string{"user"},
				Roles: []string{"qs:evaluator", "qs:staff"}, RequireDelegatedActorOnGrant: true,
			},
		},
	})
	require.NoError(t, err)
	commands := &assignmentCommandsFake{}
	srv := &authorizationServer{assignments: commands, assignmentAdmission: policy}

	grantResponse, err := srv.GrantAssignment(serviceContext("qs-apiserver.svc"), &authzv3.GrantAssignmentRequest{
		Subject: "user:100", Domain: "fangcun", RoleName: "qs:evaluator", GrantedBy: "user:1",
	})
	require.NoError(t, err)
	require.Len(t, commands.grants, 1)
	require.EqualValues(t, 11, grantResponse.PolicyVersion)

	revokeResponse, err := srv.RevokeAssignment(serviceContext("qs-apiserver.svc"), &authzv3.RevokeAssignmentRequest{
		Subject: "user:100", Domain: "fangcun", RoleName: "qs:evaluator",
	})
	require.NoError(t, err)
	require.Equal(t, "service:qs-apiserver.svc", commands.revokes[0].ChangedBy)
	require.EqualValues(t, 12, revokeResponse.PolicyVersion)

	replaceResponse, err := srv.ReplaceManagedAssignments(serviceContext("qs-apiserver.svc"), &authzv3.ReplaceManagedAssignmentsRequest{
		Subject: "user:100", Domain: "fangcun", RoleNames: []string{"qs:evaluator", "qs:staff"},
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

	_, err := srv.GrantAssignment(ctx, &authzv3.GrantAssignmentRequest{
		Subject: "user:100", Domain: "fangcun", RoleName: "qs:evaluator", GrantedBy: "user:1",
	})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = srv.RevokeAssignment(ctx, &authzv3.RevokeAssignmentRequest{
		Subject: "user:100", Domain: "fangcun", RoleName: "qs:evaluator",
	})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = srv.ReplaceManagedAssignments(ctx, &authzv3.ReplaceManagedAssignmentsRequest{
		Subject: "user:100", Domain: "fangcun", RoleNames: []string{"qs:evaluator"}, ChangedBy: "user:1",
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
		{name: "evaluator plan", subject: "user:2", originType: "plan", allowed: false},
		{name: "plan manager adhoc", subject: "user:3", originType: "adhoc", allowed: false},
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
				require.Equal(t, authzv3.DecisionReason_ALLOWED, response.GetReason())
				require.NotEmpty(t, response.GetMatchedGrantId())
				require.NotEmpty(t, response.GetMatchedRole())
			} else {
				require.Equal(t, authzv3.DecisionReason_NOT_MATCHED, response.GetReason())
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

func assessmentCheckRequest(subjectKey, originType string) *authzv3.CheckRequest {
	return &authzv3.CheckRequest{
		Subject: subjectKey, Domain: "fangcun", Resource: assessmentResource, Action: "retry",
		ObjectContext: &authzv3.ObjectContext{ObjectId: "assessment-1", Attributes: []*authzv3.ObjectAttribute{{
			Key:   attribute.ObjectOriginType,
			Value: &authzv3.ObjectAttribute_StringValue{StringValue: originType},
		}}},
	}
}

type staticRuntimeSource struct{ dataset authzruntime.Dataset }

func (s staticRuntimeSource) Load(context.Context) (authzruntime.Dataset, error) {
	return s.dataset, nil
}

func newMatrixRuntime(t *testing.T) *authzruntime.Runtime {
	t.Helper()
	assessment, err := resource.NewResource(
		assessmentResource,
		[]string{"retry", "force_retry", "batch_evaluate"},
		resource.WithID(resource.NewResourceID(20)),
		resource.WithDisplayName("Assessments"),
		resource.WithAttributeSchema(attribute.AssessmentSchema()),
	)
	require.NoError(t, err)
	adhoc, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("adhoc")))
	require.NoError(t, err)
	plan, err := constraint.New(constraint.Equal(attribute.ObjectOriginType, constraint.StringValue("plan")))
	require.NoError(t, err)
	admin, err := permissiongrant.NewSystem(
		meta.FromUint64(11), "fangcun", resource.ResourceID{}, "qs:*:*:*", "*", constraint.Empty(), "contract-test",
	)
	require.NoError(t, err)
	evaluator, err := permissiongrant.New(
		meta.FromUint64(12), "fangcun", assessment.ID, assessment.KeyString(), "retry", adhoc, "contract-test",
	)
	require.NoError(t, err)
	planManager, err := permissiongrant.New(
		meta.FromUint64(13), "fangcun", assessment.ID, assessment.KeyString(), "retry", plan, "contract-test",
	)
	require.NoError(t, err)
	admin.ID, evaluator.ID, planManager.ID = meta.FromUint64(100), meta.FromUint64(102), meta.FromUint64(103)

	runtime, err := authzruntime.NewRuntime(context.Background(), staticRuntimeSource{dataset: authzruntime.Dataset{
		Roles: []authzruntime.RoleRecord{
			{ID: meta.FromUint64(11), TenantID: "fangcun", Name: "qs:admin"},
			{ID: meta.FromUint64(12), TenantID: "fangcun", Name: "qs:evaluator"},
			{ID: meta.FromUint64(13), TenantID: "fangcun", Name: "qs:evaluation_plan_manager"},
			{ID: meta.FromUint64(14), TenantID: "fangcun", Name: "qs:staff"},
		},
		Assignments: []authzruntime.AssignmentRecord{
			{TenantID: "fangcun", SubjectKey: "user:1", RoleID: meta.FromUint64(11)},
			{TenantID: "fangcun", SubjectKey: "user:2", RoleID: meta.FromUint64(12)},
			{TenantID: "fangcun", SubjectKey: "user:3", RoleID: meta.FromUint64(13)},
			{TenantID: "fangcun", SubjectKey: "user:4", RoleID: meta.FromUint64(14)},
		},
		Grants:    []*permissiongrant.Grant{&admin, &evaluator, &planManager},
		Resources: []*resource.Resource{&assessment},
		Versions:  map[string]int64{"fangcun": 41},
	}}, authorization.NewEvaluator())
	require.NoError(t, err)
	return runtime
}

func newAuthorizationTestClient(t *testing.T, checker authorizationChecker) (authzv3.AuthorizationServiceClient, func()) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(
		ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (any, error) {
		return handler(interceptors.ContextWithServiceIdentity(ctx, &interceptors.ServiceIdentity{
			ServiceName: objectattributeadmission.TrustedAssessmentAttributeService,
		}), req)
	}))
	authzv3.RegisterAuthorizationServiceServer(server, &authorizationServer{
		checker: checker,
		objectAttributeAdmission: objectattributeadmission.NewDefaultPolicy(),
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
	return authzv3.NewAuthorizationServiceClient(connection), cleanup
}

type checkerFake struct {
	decision authorization.Decision
	err      error
	calls    []authorization.Request
}

func (f *checkerFake) Check(_ context.Context, request authorization.Request) (authorization.Decision, error) {
	f.calls = append(f.calls, request)
	if f.err != nil {
		return authorization.Decision{}, f.err
	}
	return f.decision, nil
}

type snapshotReaderFake struct {
	snapshot authzapp.SubjectSnapshot
	err      error
}

func (f *snapshotReaderFake) Read(_ context.Context, _ subject.Ref, _, _ string) (authzapp.SubjectSnapshot, error) {
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
