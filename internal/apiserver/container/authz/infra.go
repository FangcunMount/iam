package authz

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	permissionGrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	policyDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/rolebinding"
	roleInheritanceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	nativeInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/native"
	permissionGrantInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	policyInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	resourceInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	roleInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	bindingInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/rolebinding"
	roleInheritanceInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	mysqlAuthzUow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/pkg/event"
)

type authzInfrastructureComponents struct {
	nativeRuntime *nativeInfra.Runtime

	roleRepository            roleDomain.Repository
	bindingRepository         bindingDomain.Repository
	resourceRepository        resourceDomain.Repository
	policyVersionRepository   policyDomain.Repository
	permissionGrantRepository permissionGrantDomain.Repository
	roleInheritanceRepository roleInheritanceDomain.Repository
	unitOfWork                authzuow.UnitOfWork
}

func (m *AuthzModule) initializeInfrastructure(
	db *gorm.DB,
	eventStager event.Stager,
	userResolver useraccess.UserResolver,
) (*authzInfrastructureComponents, error) {
	nativeRuntime, err := nativeInfra.NewRuntime(context.Background(), nativeInfra.NewMySQLSource(db))
	if err != nil {
		return nil, fmt.Errorf("failed to create native authorization runtime: %w", err)
	}

	return &authzInfrastructureComponents{
		nativeRuntime:             nativeRuntime,
		roleRepository:            roleInfra.NewRoleRepository(db),
		bindingRepository:         bindingInfra.NewBindingRepository(db),
		resourceRepository:        resourceInfra.NewResourceRepository(db),
		policyVersionRepository:   policyInfra.NewPolicyVersionRepository(db),
		permissionGrantRepository: permissionGrantInfra.NewRepository(db),
		roleInheritanceRepository: roleInheritanceInfra.NewRepository(db),
		unitOfWork:                mysqlAuthzUow.NewUnitOfWork(db, userResolver, eventStager),
	}, nil
}
