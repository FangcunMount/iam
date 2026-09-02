package roleinheritance_test

import (
	"context"
	"errors"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	roleInheritanceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/roleinheritance"
	authztestutil "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/testutil"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/stretchr/testify/require"
)

func TestRoleInheritanceCreateRejectsCyclesAndRevokeAdvancesPolicy(t *testing.T) {
	fixture, service, stager := setupRoleInheritanceService(t, nil)
	roles := fixture.Roles
	child := seedRole(t, roles, "qs:operator", "tenant-a")
	parent := seedRole(t, roles, "qs:evaluator", "tenant-a")

	created, err := service.Create(context.Background(), roleInheritanceApp.CreateCommand{
		TenantID: "tenant-a", RoleID: child.ID, InheritedRoleID: parent.ID, GrantedBy: "operator-1",
	})
	require.NoError(t, err)
	require.False(t, created.ID.IsZero())
	require.Len(t, stager.events, 1)

	_, err = service.Create(context.Background(), roleInheritanceApp.CreateCommand{
		TenantID: "tenant-a", RoleID: parent.ID, InheritedRoleID: child.ID, GrantedBy: "operator-1",
	})
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))

	items, err := service.List(context.Background(), "tenant-a", child.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NoError(t, service.Revoke(context.Background(), roleInheritanceApp.RevokeCommand{
		TenantID: "tenant-a", ID: created.ID, RevokedBy: "operator-1",
	}))
	items, err = service.List(context.Background(), "tenant-a", meta.ID(0))
	require.NoError(t, err)
	require.Empty(t, items)
	require.Len(t, stager.events, 2)

	require.EqualValues(t, 2, fixture.PolicyVersionCount(t))
}

func TestRoleInheritanceRevokeAlreadyRevokedReturnsError(t *testing.T) {
	fixture, service, stager := setupRoleInheritanceService(t, nil)
	roles := fixture.Roles
	child := seedRole(t, roles, "qs:operator", "tenant-a")
	parent := seedRole(t, roles, "qs:evaluator", "tenant-a")
	created, err := service.Create(context.Background(), roleInheritanceApp.CreateCommand{
		TenantID: "tenant-a", RoleID: child.ID, InheritedRoleID: parent.ID, GrantedBy: "operator-1",
	})
	require.NoError(t, err)
	require.NoError(t, service.Revoke(context.Background(), roleInheritanceApp.RevokeCommand{
		TenantID: "tenant-a", ID: created.ID, RevokedBy: "operator-1",
	}))
	require.Len(t, stager.events, 2)

	err = service.Revoke(context.Background(), roleInheritanceApp.RevokeCommand{
		TenantID: "tenant-a", ID: created.ID, RevokedBy: "operator-1",
	})
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
	require.Len(t, stager.events, 2, "duplicate revoke must not publish another policy version")
	require.EqualValues(t, 2, fixture.PolicyVersionCount(t))
}

func TestRoleInheritanceCreateRejectsUnknownOrCrossTenantRole(t *testing.T) {
	fixture, service, _ := setupRoleInheritanceService(t, nil)
	roles := fixture.Roles
	child := seedRole(t, roles, "qs:operator", "tenant-a")
	otherTenantParent := seedRole(t, roles, "qs:evaluator", "tenant-b")

	_, err := service.Create(context.Background(), roleInheritanceApp.CreateCommand{
		TenantID: "tenant-a", RoleID: child.ID, InheritedRoleID: otherTenantParent.ID, GrantedBy: "operator-1",
	})
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
}

func TestRoleInheritanceCreateRollsBackWhenPolicyEventCannotBeStaged(t *testing.T) {
	fixture, service, _ := setupRoleInheritanceService(t, failingStager{})
	roles := fixture.Roles
	child := seedRole(t, roles, "qs:operator", "tenant-a")
	parent := seedRole(t, roles, "qs:evaluator", "tenant-a")

	_, err := service.Create(context.Background(), roleInheritanceApp.CreateCommand{
		TenantID: "tenant-a", RoleID: child.ID, InheritedRoleID: parent.ID, GrantedBy: "operator-1",
	})
	require.Error(t, err)

	require.Zero(t, fixture.RoleInheritanceCount(t))
	require.Zero(t, fixture.PolicyVersionCount(t))
}

func setupRoleInheritanceService(t *testing.T, override event.Stager) (*authztestutil.Fixture, *roleInheritanceApp.Service, *recordingStager) {
	t.Helper()
	recording := &recordingStager{}
	stager := event.Stager(recording)
	if override != nil {
		stager = override
	}
	fixture := authztestutil.NewFixture(t, stager)
	service := roleInheritanceApp.NewService(fixture.UnitOfWork, fixture.RoleInheritances, nil)
	return fixture, service, recording
}

func seedRole(t *testing.T, repository roleDomain.Repository, name, tenantID string) roleDomain.Role {
	t.Helper()
	role, err := roleDomain.NewRole(name, name, tenantID)
	require.NoError(t, err)
	require.NoError(t, repository.Create(context.Background(), &role))
	return role
}

type recordingStager struct{ events []event.DomainEvent }

func (s *recordingStager) Stage(_ context.Context, events ...event.DomainEvent) error {
	s.events = append(s.events, events...)
	return nil
}

type failingStager struct{}

func (failingStager) Stage(context.Context, ...event.DomainEvent) error {
	return errors.New("outbox unavailable")
}
