package authz

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	authzv2 "github.com/FangcunMount/iam/v2/api/grpc/iam/authz/v2"
	authzapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/authorization"
	rolebindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
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
			Subject: "user:1", Domain: "tenant-a", Object: "iam:user:*", Action: "read",
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
		checker := &authzCheckerFake{decision: authzDomain.AuthorizationDecision{Allowed: true}}
		srv := &authorizationServer{checker: checker}

		resp, err := srv.Check(context.Background(), &authzv2.CheckRequest{
			Subject: "user:1", Domain: "tenant-a", Object: "iam:user:*", Action: "read", ScopeType: "origin", ScopeValue: "1",
		})

		require.NoError(t, err)
		require.True(t, resp.Allowed)
		require.Len(t, checker.calls, 1)
		require.Equal(t, authzDomain.SubjectTypeUser, checker.calls[0].Subject.Type)
		require.Equal(t, "1", checker.calls[0].Subject.ID)
		require.Equal(t, "tenant-a", checker.calls[0].TenantID)
		require.Equal(t, "iam:user:*", checker.calls[0].ResourceKey)
		require.Equal(t, "read", checker.calls[0].Action)
		require.Equal(t, authzDomain.Scope{Kind: authzDomain.ScopeKindOrigin, Value: "1"}, checker.calls[0].ObjectScope)
	})

	t.Run("check error", func(t *testing.T) {
		t.Parallel()
		srv := &authorizationServer{checker: &authzCheckerFake{err: errors.New("boom")}}

		_, err := srv.Check(context.Background(), &authzv2.CheckRequest{
			Subject: "user:1", Domain: "tenant-a", Object: "iam:user:*", Action: "read",
		})

		require.Equal(t, codes.Internal, status.Code(err))
	})
}

func TestAuthorizationServerSnapshotUsesApplicationReader(t *testing.T) {
	t.Parallel()

	reader := &authzSnapshotReaderFake{
		snapshot: &authzapp.Snapshot{
			Roles: []string{"iam:admin"},
			Permissions: []authzapp.PermissionEntry{
				{ResourceKey: "iam:user:*", Action: "read"},
				{ResourceKey: "iam:user:*", Action: "write"},
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
		{Resource: "iam:user:*", Action: "read", ScopeType: "all", ScopeValue: "*"},
		{Resource: "iam:user:*", Action: "write", ScopeType: "all", ScopeValue: "*"},
	}, resp.Permissions)
	require.Len(t, reader.calls, 1)
	require.Equal(t, authzDomain.SubjectTypeUser, reader.calls[0].Subject.Type)
	require.Equal(t, "1", reader.calls[0].Subject.ID)
	require.Equal(t, "tenant-a", reader.calls[0].TenantID)
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
		srv := &authorizationServer{snapshotReader: &authzSnapshotReaderFake{err: errors.New("snapshot failed")}}

		_, err := srv.GetAuthorizationSnapshot(context.Background(), &authzv2.GetAuthorizationSnapshotRequest{
			Subject: "user:1", Domain: "tenant-a", AppName: "iam",
		})

		require.Equal(t, codes.Internal, status.Code(err))
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
		Subject:   authzDomain.Subject{Type: authzDomain.SubjectTypeUser, ID: "100"},
		TenantID:  "tenant-a",
		RoleName:  "iam:admin",
		GrantedBy: "operator-1",
	}}, commands.grants)

	_, err = srv.RevokeAssignment(context.Background(), &authzv2.RevokeAssignmentRequest{
		Subject:  "user:100",
		Domain:   "tenant-a",
		RoleName: "iam:admin",
	})
	require.NoError(t, err)
	require.Equal(t, []rolebindingApp.RevokeByRoleNameCommand{{
		Subject:  authzDomain.Subject{Type: authzDomain.SubjectTypeUser, ID: "100"},
		TenantID: "tenant-a",
		RoleName: "iam:admin",
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
		srv := &authorizationServer{
			roleBindings: &roleBindingCommandsFake{grantErr: errors.New("grant failed")},
		}

		_, err := srv.GrantAssignment(context.Background(), &authzv2.GrantAssignmentRequest{
			Subject: "user:100", Domain: "tenant-a", RoleName: "iam:admin",
		})

		require.Equal(t, codes.Internal, status.Code(err))
	})
}

type authzCheckerFake struct {
	decision authzDomain.AuthorizationDecision
	err      error
	calls    []authzapp.CheckCommand
}

func (f *authzCheckerFake) Check(_ context.Context, cmd authzapp.CheckCommand) (authzDomain.AuthorizationDecision, error) {
	f.calls = append(f.calls, cmd)
	if f.err != nil {
		return authzDomain.AuthorizationDecision{}, f.err
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
