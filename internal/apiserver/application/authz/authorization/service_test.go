package authorization

import (
	"context"
	"errors"
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/decision"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/permission"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/scope"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestCheckerDelegatesToDecisionEngine(t *testing.T) {
	t.Parallel()

	subject, err := subject.NewRef(subject.TypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	engine := &decisionEngineFake{decision: decision.Decision{Allowed: true}}
	checker := NewChecker(engine, &versionRepoFake{version: &policyDomain.PolicyVersion{Version: 11}})

	cmd, err := NewCheckCommand(subject, "tenant-a", "iam:identity:collection:users", "read", scope.Default())
	require.NoError(t, err)
	decision, err := checker.Check(context.Background(), cmd)

	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, int64(11), decision.PolicyVersion)
	require.Len(t, engine.requests, 1)
	require.Equal(t, subject, engine.requests[0].Subject)
	require.Equal(t, "tenant-a", engine.requests[0].TenantIDString())
	require.Equal(t, "iam:identity:collection:users", engine.requests[0].ResourceKeyString())
	require.Equal(t, "read", engine.requests[0].ActionString())
	require.Equal(t, scope.Default(), engine.requests[0].ObjectScope)
}

func TestSnapshotReaderFiltersAndDeduplicatesByApp(t *testing.T) {
	t.Parallel()

	subject, err := subject.NewRef(subject.TypeUser, meta.FromUint64(100))
	require.NoError(t, err)
	reader := NewSnapshotReader(
		&snapshotStoreFake{
			roles: []string{"iam:admin", "qs:admin", "iam:admin"},
			permissions: []permission.Permission{
				mustPermission(t, "iam:admin", "tenant-a", "iam:identity:collection:users", "read"),
				mustPermission(t, "iam:admin", "tenant-a", "qs:course:collection:*", "read"),
				mustPermission(t, "iam:admin", "tenant-a", "iam:identity:collection:users", "read"),
				mustPermission(t, "iam:admin", "tenant-a", "iam:identity:collection:users", "write"),
			},
		},
		&versionRepoFake{version: &policyDomain.PolicyVersion{Version: 9}},
	)

	query, err := NewSnapshotQuery(subject, "tenant-a", "iam")
	require.NoError(t, err)
	snapshot, err := reader.Read(context.Background(), query)

	require.NoError(t, err)
	require.Equal(t, []string{"iam:admin"}, snapshot.Roles)
	require.Equal(t, []PermissionEntry{
		{ResourceKey: "iam:identity:collection:users", Action: "read", Scope: scope.Default()},
		{ResourceKey: "iam:identity:collection:users", Action: "write", Scope: scope.Default()},
	}, snapshot.Permissions)
	require.Equal(t, int64(9), snapshot.AuthzVersion)
}

func TestSnapshotReaderValidatesDependencies(t *testing.T) {
	t.Parallel()

	subject, err := subject.NewRef(subject.TypeUser, meta.FromUint64(100))
	require.NoError(t, err)

	query, err := NewSnapshotQuery(subject, "tenant-a", "iam")
	require.NoError(t, err)

	_, err = NewSnapshotReader(nil, &versionRepoFake{}).Read(context.Background(), query)
	require.Error(t, err)

	_, err = NewSnapshotReader(&snapshotStoreFake{}, nil).Read(context.Background(), query)
	require.Error(t, err)

	_, err = NewSnapshotReader(&snapshotStoreFake{}, &versionRepoFake{}).Read(context.Background(), SnapshotQuery{
		Subject:  subject,
		TenantID: query.TenantID,
	})
	require.Error(t, err)
}

func TestSnapshotQueryUsesTenantValueObject(t *testing.T) {
	t.Parallel()

	subject, err := subject.NewRef(subject.TypeUser, meta.FromUint64(100))
	require.NoError(t, err)

	query, err := NewSnapshotQuery(subject, " tenant-a ", " iam ")

	require.NoError(t, err)
	require.Equal(t, "tenant-a", query.TenantIDString())
	require.Equal(t, "iam", query.AppName)

	_, err = NewSnapshotQuery(subject, "", "iam")
	require.Error(t, err)

	_, err = NewSnapshotQuery(subject, "tenant-a", "")
	require.Error(t, err)
}

type decisionEngineFake struct {
	decision decision.Decision
	err      error
	requests []decision.Request
}

func (f *decisionEngineFake) Check(_ context.Context, request decision.Request) (decision.Decision, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return decision.Decision{}, f.err
	}
	return f.decision, nil
}

type snapshotStoreFake struct {
	roles       []string
	permissions []permission.Permission
}

func (f *snapshotStoreFake) RoleNamesForSubject(context.Context, subject.Ref, string) ([]string, error) {
	return f.roles, nil
}

func (f *snapshotStoreFake) PermissionsForSubject(context.Context, subject.Ref, string) ([]permission.Permission, error) {
	return f.permissions, nil
}

type versionRepoFake struct {
	version *policyDomain.PolicyVersion
	err     error
}

func (f *versionRepoFake) GetOrCreate(context.Context, string) (*policyDomain.PolicyVersion, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.version == nil {
		return nil, errors.New("missing version")
	}
	return f.version, nil
}
func (f *versionRepoFake) Increment(context.Context, string, string, string) (*policyDomain.PolicyVersion, error) {
	return f.version, nil
}
func (f *versionRepoFake) GetCurrent(context.Context, string) (*policyDomain.PolicyVersion, error) {
	return f.version, nil
}

func mustPermission(t *testing.T, roleName, tenantID, resourceKey, action string) permission.Permission {
	t.Helper()
	permission, err := permission.New(roleName, tenantID, resourceKey, action)
	require.NoError(t, err)
	return permission
}
