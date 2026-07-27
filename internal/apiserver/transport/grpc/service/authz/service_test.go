package authz

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authzv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authz/v2"
	assignmentauth "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/assignmentauth"
	authzapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/authorization"
	rolebindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/decision"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAuthorizationServerCheckBranches(t *testing.T) {
	t.Parallel()

	t.Run("engine unavailable", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{}

		_, err := srv.Check(context.Background(), &authzv2.CheckRequest{
			Subject: "user:1", Domain: "tenant-a", Object: "iam:identity:collection:users", Action: "read",
		})

		require.Equal(t, codes.Unavailable, status.Code(err))
	})

	t.Run("invalid argument", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{checker: &authzCheckerFake{}}

		_, err := srv.Check(context.Background(), &authzv2.CheckRequest{})

		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("check success", func(t *testing.T) {
		t.Parallel()
		checker := &authzCheckerFake{decision: decision.Decision{
			Allowed:       true,
			Reason:        "allowed",
			PolicyVersion: 12,
		}}
		srv := &authorizationServer{checker: checker}

		resp, err := srv.Check(context.Background(), &authzv2.CheckRequest{
			Subject: "user:1", Domain: "tenant-a", Object: "iam:identity:collection:users", Action: "read", ScopeType: "origin", ScopeValue: "1",
		})

		require.NoError(t, err)
		require.True(t, resp.Allowed)
		require.Equal(t, "allowed", resp.Reason)
		require.Equal(t, int64(12), resp.PolicyVersion)
		require.Len(t, checker.calls, 1)
		require.Equal(t, subject.TypeUser, checker.calls[0].Subject.Type)
		require.Equal(t, meta.FromUint64(1), checker.calls[0].Subject.ID)
		require.Equal(t, "tenant-a", checker.calls[0].TenantIDString())
		require.Equal(t, "iam:identity:collection:users", checker.calls[0].ResourceKeyString())
		require.Equal(t, "read", checker.calls[0].ActionString())
		require.Equal(t, scope.Scope{Kind: scope.KindOrigin, Value: "1"}, checker.calls[0].ObjectScope)
	})

	t.Run("check error", func(t *testing.T) {
		t.Parallel()
		const sentinel = "authz-check-internal-sentinel"
		srv := &authorizationServer{checker: &authzCheckerFake{err: errors.New(sentinel)}}

		_, err := srv.Check(context.Background(), &authzv2.CheckRequest{
			Subject: "user:1", Domain: "tenant-a", Object: "iam:identity:collection:users", Action: "read",
		})

		require.Equal(t, codes.Internal, status.Code(err))
		require.Equal(t, "internal server error", status.Convert(err).Message())
		require.NotContains(t, err.Error(), sentinel)
		require.NotContains(t, err.Error(), "enforce")
	})
}

func TestAuthorizationServerSnapshotUsesApplicationReader(t *testing.T) {
	t.Parallel()

	reader := &authzSnapshotReaderFake{
		snapshot: &authzapp.Snapshot{
			Roles: []string{"iam:admin"},
			Permissions: []authzapp.PermissionEntry{
				{ResourceKey: "iam:identity:collection:users", Action: "read"},
				{ResourceKey: "iam:identity:collection:users", Action: "write"},
			},
			AuthzVersion: 12,
		},
	}
	srv := &authorizationServer{snapshotReader: reader}

	resp, err := srv.GetAuthorizationSnapshot(context.Background(), &authzv2.GetAuthorizationSnapshotRequest{
		Subject: "user:1",
		Domain:  "tenant-a",
		AppName: "iam",
	})

	require.NoError(t, err)
	require.Equal(t, []string{"iam:admin"}, resp.Roles)
	require.Equal(t, int64(12), resp.AuthzVersion)
	require.Equal(t, []*authzv2.PermissionEntry{
		{Resource: "iam:identity:collection:users", Action: "read", ScopeType: "all", ScopeValue: "*"},
		{Resource: "iam:identity:collection:users", Action: "write", ScopeType: "all", ScopeValue: "*"},
	}, resp.Permissions)
	require.Len(t, reader.calls, 1)
	require.Equal(t, subject.TypeUser, reader.calls[0].Subject.Type)
	require.Equal(t, meta.FromUint64(1), reader.calls[0].Subject.ID)
	require.Equal(t, "tenant-a", reader.calls[0].TenantIDString())
	require.Equal(t, "iam", reader.calls[0].AppName)
}

func TestAuthorizationServerSnapshotBranches(t *testing.T) {
	t.Parallel()

	t.Run("reader unavailable", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{}

		_, err := srv.GetAuthorizationSnapshot(context.Background(), &authzv2.GetAuthorizationSnapshotRequest{
			Subject: "user:1", Domain: "tenant-a", AppName: "iam",
		})

		require.Equal(t, codes.Unavailable, status.Code(err))
	})

	t.Run("invalid argument", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{snapshotReader: &authzSnapshotReaderFake{}}

		_, err := srv.GetAuthorizationSnapshot(context.Background(), &authzv2.GetAuthorizationSnapshotRequest{})

		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("application error", func(t *testing.T) {
		t.Parallel()
		const sentinel = "authz-snapshot-internal-sentinel"
		srv := &authorizationServer{snapshotReader: &authzSnapshotReaderFake{err: errors.New(sentinel)}}

		_, err := srv.GetAuthorizationSnapshot(context.Background(), &authzv2.GetAuthorizationSnapshotRequest{
			Subject: "user:1", Domain: "tenant-a", AppName: "iam",
		})

		require.Equal(t, codes.Internal, status.Code(err))
		require.Equal(t, "internal server error", status.Convert(err).Message())
		require.NotContains(t, err.Error(), sentinel)
		require.NotContains(t, err.Error(), "get authorization snapshot")
	})
}

func TestAuthorizationServerGrantAndRevokeAssignment(t *testing.T) {
	t.Parallel()

	commands := &roleBindingCommandsFake{}
	srv := &authorizationServer{roleBindings: commands}

	_, err := srv.GrantAssignment(context.Background(), &authzv2.GrantAssignmentRequest{
		Subject:   "user:100",
		Domain:    "tenant-a",
		RoleName:  "iam:admin",
		GrantedBy: "operator-1",
	})
	require.NoError(t, err)
	require.Equal(t, []rolebindingApp.GrantByRoleNameCommand{{
		Subject:   subject.Ref{Type: subject.TypeUser, ID: meta.FromUint64(100)},
		TenantID:  "tenant-a",
		RoleName:  "iam:admin",
		GrantedBy: "operator-1",
	}}, commands.grants)

	_, err = srv.RevokeAssignment(context.Background(), &authzv2.RevokeAssignmentRequest{
		Subject:   "user:100",
		Domain:    "tenant-a",
		RoleName:  "iam:admin",
		RevokedBy: "operator-2",
		Reason:    "manual revoke",
	})
	require.NoError(t, err)
	require.Equal(t, []rolebindingApp.RevokeByRoleNameCommand{{
		Subject:   subject.Ref{Type: subject.TypeUser, ID: meta.FromUint64(100)},
		TenantID:  "tenant-a",
		RoleName:  "iam:admin",
		ChangedBy: "operator-2",
		Reason:    "manual revoke",
	}}, commands.revokes)
}

func TestAuthorizationServerAssignmentErrors(t *testing.T) {
	t.Parallel()

	t.Run("service unavailable", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{}

		_, err := srv.GrantAssignment(context.Background(), &authzv2.GrantAssignmentRequest{
			Subject: "user:100", Domain: "tenant-a", RoleName: "iam:admin",
		})

		require.Equal(t, codes.Unavailable, status.Code(err))
	})

	t.Run("invalid subject", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{roleBindings: &roleBindingCommandsFake{}}

		_, err := srv.GrantAssignment(context.Background(), &authzv2.GrantAssignmentRequest{
			Subject: "malformed", Domain: "tenant-a", RoleName: "iam:admin",
		})

		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("role not found", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{
			roleBindings: &roleBindingCommandsFake{
				grantErr: perrors.WithCode(code.ErrRoleNotFound, "role not found"),
			},
		}

		_, err := srv.GrantAssignment(context.Background(), &authzv2.GrantAssignmentRequest{
			Subject: "user:100", Domain: "tenant-a", RoleName: "iam:admin",
		})

		require.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("application error", func(t *testing.T) {
		t.Parallel()
		const sentinel = "authz-grant-internal-sentinel"
		srv := &authorizationServer{
			roleBindings: &roleBindingCommandsFake{grantErr: errors.New(sentinel)},
		}

		_, err := srv.GrantAssignment(context.Background(), &authzv2.GrantAssignmentRequest{
			Subject: "user:100", Domain: "tenant-a", RoleName: "iam:admin",
		})

		require.Equal(t, codes.Internal, status.Code(err))
		require.Equal(t, "internal server error", status.Convert(err).Message())
		require.NotContains(t, err.Error(), sentinel)
	})
}

func TestAuthorizationServerAssignmentRequestAuthorization(t *testing.T) {
	t.Parallel()

	authorizer, err := assignmentauth.New(assignmentauth.Config{
		DefaultPolicy: "deny",
		Services: map[string]assignmentauth.ServiceConstraint{
			"qs-apiserver.svc": {
				Domains:                      []string{"fangcun"},
				SubjectTypes:                 []string{"user"},
				Roles:                        []string{"qs:admin"},
				RequireDelegatedActorOnGrant: true,
			},
			"admin": {AllowAll: true},
		},
	})
	require.NoError(t, err)

	t.Run("missing service identity is denied", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{
			roleBindings:         &roleBindingCommandsFake{},
			assignmentAuthorizer: authorizer,
		}
		_, err := srv.GrantAssignment(context.Background(), &authzv2.GrantAssignmentRequest{
			Subject: "user:100", Domain: "fangcun", RoleName: "qs:admin", GrantedBy: "user:1",
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("service constraints are enforced", func(t *testing.T) {
		t.Parallel()
		commands := &roleBindingCommandsFake{}
		srv := &authorizationServer{
			roleBindings:         commands,
			assignmentAuthorizer: authorizer,
		}
		ctx := interceptors.ContextWithServiceIdentity(context.Background(), &interceptors.ServiceIdentity{
			ServiceName: "qs-apiserver.svc",
		})

		_, err := srv.GrantAssignment(ctx, &authzv2.GrantAssignmentRequest{
			Subject: "user:100", Domain: "fangcun", RoleName: "qs:unknown", GrantedBy: "user:1",
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.Empty(t, commands.grants)

		_, err = srv.GrantAssignment(ctx, &authzv2.GrantAssignmentRequest{
			Subject: "user:100", Domain: "fangcun", RoleName: "qs:admin", GrantedBy: "user:1",
		})
		require.NoError(t, err)
		require.Len(t, commands.grants, 1)
	})

	t.Run("revoke audit actor falls back to caller service", func(t *testing.T) {
		t.Parallel()
		commands := &roleBindingCommandsFake{}
		srv := &authorizationServer{
			roleBindings:         commands,
			assignmentAuthorizer: authorizer,
		}
		ctx := interceptors.ContextWithServiceIdentity(context.Background(), &interceptors.ServiceIdentity{
			ServiceName: "qs-apiserver.svc",
		})

		_, err := srv.RevokeAssignment(ctx, &authzv2.RevokeAssignmentRequest{
			Subject: "user:100", Domain: "fangcun", RoleName: "qs:admin",
		})
		require.NoError(t, err)
		require.Equal(t, "service:qs-apiserver.svc", commands.revokes[0].ChangedBy)
	})

	t.Run("authorizer failure is internal", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{
			roleBindings:         &roleBindingCommandsFake{},
			assignmentAuthorizer: assignmentAuthorizerErrorStub{},
		}
		ctx := interceptors.ContextWithServiceIdentity(context.Background(), &interceptors.ServiceIdentity{
			ServiceName: "qs-apiserver.svc",
		})
		_, err := srv.GrantAssignment(ctx, &authzv2.GrantAssignmentRequest{
			Subject: "user:100", Domain: "fangcun", RoleName: "qs:admin", GrantedBy: "user:1",
		})
		require.Equal(t, codes.Internal, status.Code(err))
	})
}

type assignmentAuthorizerErrorStub struct{}

func (assignmentAuthorizerErrorStub) AuthorizeAssignment(assignmentauth.Request) error {
	return errors.New("constraint repository unavailable")
}

type authzCheckerFake struct {
	decision decision.Decision
	err      error
	calls    []authzapp.CheckCommand
}

func (f *authzCheckerFake) Check(_ context.Context, cmd authzapp.CheckCommand) (decision.Decision, error) {
	f.calls = append(f.calls, cmd)
	if f.err != nil {
		return decision.Decision{}, f.err
	}
	return f.decision, nil
}

type authzSnapshotReaderFake struct {
	snapshot *authzapp.Snapshot
	err      error
	calls    []authzapp.SnapshotQuery
}

func (f *authzSnapshotReaderFake) Read(_ context.Context, query authzapp.SnapshotQuery) (*authzapp.Snapshot, error) {
	f.calls = append(f.calls, query)
	if f.err != nil {
		return nil, f.err
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
