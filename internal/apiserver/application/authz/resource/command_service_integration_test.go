package resource_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	authztestutil "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/testutil"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	permissiongrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	authzfixture "github.com/FangcunMount/iam/v3/internal/apiserver/testfixtures/assessment"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/stretchr/testify/require"
)

func TestUpdateResourceRejectsCandidateThatInvalidatesActiveGrant(t *testing.T) {
	db, catalog, resources, grants, _ := setupResourceCatalog(t)
	resource := seedAssessmentResource(t, resources)
	seedGrant(t, grants, resource, "tenant-a")

	cmd, err := resourceApp.NewUpdateResourceCommand(resource.ID, nil, []string{"read"}, nil, nil)
	require.NoError(t, err)
	cmd.TenantID = "tenant-operator"
	cmd.ChangedBy = "operator"
	cmd.Actor, err = subject.NewUserRef(meta.FromUint64(1))
	require.NoError(t, err)

	_, err = catalog.UpdateResource(context.Background(), cmd)

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrResourceInUse))
	persisted, err := resources.FindByID(context.Background(), resource.ID)
	require.NoError(t, err)
	require.True(t, persisted.HasAction("retry"))
	require.Zero(t, db.PolicyVersionCount(t))
}

func TestUpdateResourceVersionsEveryTenantWithAnActiveGrant(t *testing.T) {
	_, catalog, resources, grants, stager := setupResourceCatalog(t)
	resource := seedAssessmentResource(t, resources)
	seedGrant(t, grants, resource, "tenant-b")
	seedGrant(t, grants, resource, "tenant-a")

	cmd, err := resourceApp.NewUpdateResourceCommand(resource.ID, nil, []string{"retry", "read"}, nil, nil)
	require.NoError(t, err)
	cmd.TenantID = "tenant-operator"
	cmd.ChangedBy = "operator"
	cmd.Actor, err = subject.NewUserRef(meta.FromUint64(1))
	require.NoError(t, err)

	_, err = catalog.UpdateResource(context.Background(), cmd)
	require.NoError(t, err)
	require.Len(t, stager.events, 3)
}

func setupResourceCatalog(t *testing.T) (*authztestutil.Fixture, *resourceApp.ResourceCatalog, resourceDomain.Repository, permissiongrantDomain.Repository, *recordingStager) {
	t.Helper()
	stager := &recordingStager{}
	fixture := authztestutil.NewFixture(t, stager)
	catalog := resourceApp.NewResourceCatalog(fixture.UnitOfWork, nil, allowCatalogWriter{})
	return fixture, catalog, fixture.Resources, fixture.PermissionGrants, stager
}

func seedAssessmentResource(t *testing.T, repository resourceDomain.Repository) resourceDomain.Resource {
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

func seedGrant(t *testing.T, repository permissiongrantDomain.Repository, resource resourceDomain.Resource, tenantID string) {
	t.Helper()
	grant, err := permissiongrantDomain.New(
		meta.FromUint64(17), tenantID, resource.ID, resource.KeyString(), "retry", constraint.Empty(), "operator",
	)
	require.NoError(t, err)
	require.NoError(t, repository.Create(context.Background(), &grant))
}

type recordingStager struct{ events []event.DomainEvent }

func (s *recordingStager) Stage(_ context.Context, events ...event.DomainEvent) error {
	s.events = append(s.events, events...)
	return nil
}

type allowCatalogWriter struct{}

func (allowCatalogWriter) RequireCatalogWrite(context.Context, subject.Ref, string) error { return nil }

func TestCatalogFailsClosedWithoutPlatformAuthorizer(t *testing.T) {
	fixture := authztestutil.NewFixture(t, nil)
	catalog := resourceApp.NewResourceCatalog(fixture.UnitOfWork, nil)
	actor, err := subject.NewUserRef(meta.FromUint64(1))
	require.NoError(t, err)
	_, err = catalog.CreateResource(context.Background(), resourceApp.CreateResourceCommand{Actor: actor, TenantID: "platform", ChangedBy: "1"})
	require.Error(t, err)
	require.Zero(t, fixture.PolicyVersionCount(t))
}
