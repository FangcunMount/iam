package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authzv3 "github.com/FangcunMount/iam/v3/api/grpc/iam/authz/v3"
	assignmentauth "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignmentauth"
	rolebindingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/rolebinding"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	authzruntime "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/runtime"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAuthorizationServerRequiresServiceIdentity(t *testing.T) {
	srv := &authorizationServer{checker: &checkerFake{}}
	_, err := srv.Check(context.Background(), &authzv3.CheckRequest{
		Subject: "user:1", Domain: "fangcun", Resource: assessmentResource, Action: "retry",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestAuthorizationServerCheckMapsTypedObjectContext(t *testing.T) {
	checker := &checkerFake{decision: authzruntime.Decision{
		Allowed: true, Reason: authzruntime.ReasonAllowed,
		MatchedGrantID: meta.FromUint64(100), MatchedRole: "qs:evaluator", PolicyVersion: 12,
	}}
	srv := &authorizationServer{checker: checker}
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
	srv := &authorizationServer{checker: &checkerFake{}}
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
	reader := &snapshotReaderFake{snapshot: authzruntime.SubjectSnapshot{
		Roles: []string{"qs:evaluator"}, PolicyVersion: 7,
		Permissions: []authzruntime.PermissionEntry{{
			Resource: assessmentResource, Action: "retry", Mode: authzruntime.ModeObjectCheckRequired,
		}},
	}}
	srv := &authorizationServer{snapshotReader: reader}
	resp, err := srv.GetAuthorizationSnapshot(serviceContext("qs-apiserver.svc"), &authzv3.GetAuthorizationSnapshotRequest{
		Subject: "user:1", Domain: "fangcun", AppName: "qs",
	})
	require.NoError(t, err)
	require.EqualValues(t, 7, resp.PolicyVersion)
	require.Equal(t, authzv3.AuthorizationMode_OBJECT_CHECK_REQUIRED, resp.Permissions[0].Mode)
}

func TestAuthorizationServerAssignmentsUseV3AndConstraints(t *testing.T) {
	authorizer, err := assignmentauth.New(assignmentauth.Config{
		DefaultPolicy: "deny",
		Services: map[string]assignmentauth.ServiceConstraint{
			"qs-apiserver.svc": {
				Domains: []string{"fangcun"}, SubjectTypes: []string{"user"},
				Roles: []string{"qs:evaluator"}, RequireDelegatedActorOnGrant: true,
			},
		},
	})
	require.NoError(t, err)
	commands := &roleBindingCommandsFake{}
	srv := &authorizationServer{roleBindings: commands, assignmentAuthorizer: authorizer}

	_, err = srv.GrantAssignment(serviceContext("qs-apiserver.svc"), &authzv3.GrantAssignmentRequest{
		Subject: "user:100", Domain: "fangcun", RoleName: "qs:evaluator", GrantedBy: "user:1",
	})
	require.NoError(t, err)
	require.Len(t, commands.grants, 1)

	_, err = srv.RevokeAssignment(serviceContext("qs-apiserver.svc"), &authzv3.RevokeAssignmentRequest{
		Subject: "user:100", Domain: "fangcun", RoleName: "qs:evaluator",
	})
	require.NoError(t, err)
	require.Equal(t, "service:qs-apiserver.svc", commands.revokes[0].ChangedBy)
}

type checkerFake struct {
	decision authzruntime.Decision
	err      error
	calls    []authzruntime.Request
}

func (f *checkerFake) Check(_ context.Context, request authzruntime.Request) (authzruntime.Decision, error) {
	f.calls = append(f.calls, request)
	if f.err != nil {
		return authzruntime.Decision{}, f.err
	}
	return f.decision, nil
}

type snapshotReaderFake struct {
	snapshot authzruntime.SubjectSnapshot
	err      error
}

func (f *snapshotReaderFake) Read(_ context.Context, _ subject.Ref, _, _ string) (authzruntime.SubjectSnapshot, error) {
	if f.err != nil {
		return authzruntime.SubjectSnapshot{}, f.err
	}
	return f.snapshot, nil
}

type roleBindingCommandsFake struct {
	grants    []rolebindingApp.GrantByRoleNameCommand
	revokes   []rolebindingApp.RevokeByRoleNameCommand
	grantErr  error
	revokeErr error
}

func (f *roleBindingCommandsFake) GrantByRoleName(_ context.Context, cmd rolebindingApp.GrantByRoleNameCommand) error {
	f.grants = append(f.grants, cmd)
	return f.grantErr
}

func (f *roleBindingCommandsFake) RevokeByRoleName(_ context.Context, cmd rolebindingApp.RevokeByRoleNameCommand) error {
	f.revokes = append(f.revokes, cmd)
	return f.revokeErr
}

type assignmentAuthorizerErrorStub struct{}

func (assignmentAuthorizerErrorStub) AuthorizeAssignment(assignmentauth.Request) error {
	return errors.New("constraint repository unavailable")
}

func serviceContext(serviceName string) context.Context {
	return interceptors.ContextWithServiceIdentity(context.Background(), &interceptors.ServiceIdentity{ServiceName: serviceName})
}
