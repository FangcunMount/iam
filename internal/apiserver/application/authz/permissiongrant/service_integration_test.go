package permissiongrant_test

import (
	"context"
	"testing"

	permissionGrantApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/permissiongrant"
	authztestutil "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/testutil"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	permissiongrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/assessment"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/stretchr/testify/require"
)

func TestPermissionGrantRevokeAlreadyRevokedIsIdempotentWithoutVersionBump(t *testing.T) {
	fixture, _, stager := setupPermissionGrantService(t)
	reloader := &recordingReloader{}
	service := permissionGrantApp.NewService(fixture.UnitOfWork, fixture.PermissionGrants, reloader)
	role := seedRole(t, fixture.Roles, "qs:evaluator", "tenant-a")
	resource := seedResource(t, fixture.Resources)
	grant := seedGrant(t, fixture.PermissionGrants, role, resource, "tenant-a")

	require.NoError(t, service.Revoke(context.Background(), permissionGrantApp.RevokeCommand{
		TenantID: "tenant-a", GrantID: grant.ID, RevokedBy: "operator-1",
	}))
	require.Len(t, stager.events, 1)
	require.EqualValues(t, 1, fixture.PolicyVersionCount(t))

	require.NoError(t, service.Revoke(context.Background(), permissionGrantApp.RevokeCommand{
		TenantID: "tenant-a", GrantID: grant.ID, RevokedBy: "operator-1",
	}))
	require.Len(t, stager.events, 1, "duplicate revoke must not publish another policy version")
	require.EqualValues(t, 1, fixture.PolicyVersionCount(t))
	require.Equal(t, 1, reloader.calls, "duplicate revoke must not reload an unchanged runtime policy")
}

func setupPermissionGrantService(t *testing.T) (*authztestutil.Fixture, *permissionGrantApp.Service, *recordingStager) {
	t.Helper()
	recording := &recordingStager{}
	fixture := authztestutil.NewFixture(t, recording)
	service := permissionGrantApp.NewService(fixture.UnitOfWork, fixture.PermissionGrants, nil)
	return fixture, service, recording
}

func seedRole(t *testing.T, repository roleDomain.Repository, name, tenantID string) roleDomain.Role {
	t.Helper()
	role, err := roleDomain.NewRole(name, name, tenantID)
	require.NoError(t, err)
	require.NoError(t, repository.Create(context.Background(), &role))
	return role
}

func seedResource(t *testing.T, repository resourceDomain.Repository) resourceDomain.Resource {
	t.Helper()
	resource, err := resourceDomain.NewResource(
		"qs:evaluation:collection:assessments",
		[]string{"retry"},
		resourceDomain.WithDisplayName("Assessments"),
		resourceDomain.WithAttributeSchema(authzfixture.Schema()),
	)
	require.NoError(t, err)
	require.NoError(t, repository.Create(context.Background(), &resource))
	return resource
}

func seedGrant(
	t *testing.T,
	repository permissiongrantDomain.Repository,
	role roleDomain.Role,
	resource resourceDomain.Resource,
	tenantID string,
) permissiongrantDomain.Grant {
	t.Helper()
	grant, err := permissiongrantDomain.New(
		role.ID, tenantID, resource.ID, resource.KeyString(), "retry", constraint.Empty(), "operator-1",
	)
	require.NoError(t, err)
	require.NoError(t, repository.Create(context.Background(), &grant))
	return grant
}

type recordingStager struct{ events []event.DomainEvent }

func (s *recordingStager) Stage(_ context.Context, events ...event.DomainEvent) error {
	s.events = append(s.events, events...)
	return nil
}

type recordingReloader struct{ calls int }

func (r *recordingReloader) LoadPolicy(context.Context) error {
	r.calls++
	return nil
}

func TestConditionalGrantRequiresProviderBeforeCommit(t *testing.T) {
	fixture, service, stager := setupPermissionGrantService(t)
	role := seedRole(t, fixture.Roles, "reader", "tenant-a")
	resource := seedResource(t, fixture.Resources)
	conditions, err := constraint.New(constraint.Equal("object.origin_type", constraint.StringValue("adhoc")))
	require.NoError(t, err)
	command := permissionGrantApp.CreateCommand{TenantID: "tenant-a", RoleID: role.ID, ResourceID: resource.ID, Action: "retry", Constraints: conditions, GrantedBy: "seed"}
	_, err = service.Create(context.Background(), command)
	require.Error(t, err)
	require.Zero(t, fixture.PolicyVersionCount(t))
	require.Empty(t, stager.events)
	service = permissionGrantApp.NewService(fixture.UnitOfWork, fixture.PermissionGrants, nil, authzfixture.Policy())
	_, err = service.Create(context.Background(), command)
	require.NoError(t, err)
	require.EqualValues(t, 1, fixture.PolicyVersionCount(t))
	require.Len(t, stager.events, 1)
}
