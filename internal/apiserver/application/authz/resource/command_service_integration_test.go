package resource_test

import (
	"context"
	"testing"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/attribute"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/constraint"
	permissiongrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	permissiongrantRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	policyRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	resourceRepo "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	authzUOW "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/FangcunMount/iam/v3/pkg/event"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdateResourceRejectsCandidateThatInvalidatesActiveGrant(t *testing.T) {
	db, catalog, resources, grants, _ := setupResourceCatalog(t)
	resource := seedAssessmentResource(t, resources)
	seedGrant(t, grants, resource, "tenant-a")

	cmd, err := resourceApp.NewUpdateResourceCommand(resource.ID, nil, []string{"read"}, nil, nil)
	require.NoError(t, err)
	cmd.TenantID = "tenant-operator"
	cmd.ChangedBy = "operator"

	_, err = catalog.UpdateResource(context.Background(), cmd)

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrResourceInUse))
	persisted, err := resources.FindByID(context.Background(), resource.ID)
	require.NoError(t, err)
	require.True(t, persisted.HasAction("retry"))
	var versionCount int64
	require.NoError(t, db.Model(&policyRepo.PolicyVersionPO{}).Count(&versionCount).Error)
	require.Zero(t, versionCount)
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

	_, err = catalog.UpdateResource(context.Background(), cmd)
	require.NoError(t, err)
	require.Len(t, stager.events, 3)
}

func setupResourceCatalog(t *testing.T) (*gorm.DB, *resourceApp.ResourceCatalog, resourceDomain.Repository, permissiongrantDomain.Repository, *recordingStager) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&resourceRepo.ResourcePO{}, &permissiongrantRepo.GrantPO{}, &policyRepo.PolicyVersionPO{}))
	resources := resourceRepo.NewResourceRepository(db)
	grants := permissiongrantRepo.NewRepository(db)
	stager := &recordingStager{}
	uow := authzUOW.NewUnitOfWork(db, nil, stager)
	catalog := resourceApp.NewResourceCatalog(resourceDomain.NewValidator(resources), uow, nil)
	return db, catalog, resources, grants, stager
}

func seedAssessmentResource(t *testing.T, repository resourceDomain.Repository) resourceDomain.Resource {
	t.Helper()
	resource, err := resourceDomain.NewResource(
		"qs:evaluation:collection:assessments",
		[]string{"retry"},
		resourceDomain.WithAttributeSchema(attribute.AssessmentSchema()),
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
