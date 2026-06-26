package authz

import (
	"fmt"

	policylintApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policylint"
	"gorm.io/gorm"

	authzuow "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/uow"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	bindingDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/rolebinding"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	casbinInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/casbin"
	casbinrulerepo "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/casbinrule"
	policyInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/policy"
	resourceInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/resource"
	roleInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/role"
	bindingInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/rolebinding"
	mysqlAuthzUow "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/uow/authz"
	userInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/mysql/user"
	"github.com/FangcunMount/iam/v2/pkg/event"
)

type authzInfrastructureComponents struct {
	casbinRuntime casbinInfra.RuntimeAdapters

	roleRepository          roleDomain.Repository
	bindingRepository       bindingDomain.Repository
	resourceRepository      resourceDomain.Repository
	policyVersionRepository policyDomain.Repository
	permissionFactReader    policylintApp.FactReader
	userRepository          userDomain.Repository
	unitOfWork              authzuow.UnitOfWork
}

func (m *AuthzModule) initializeInfrastructure(
	db *gorm.DB,
	eventStager event.Stager,
	modelPath string,
) (*authzInfrastructureComponents, error) {
	casbinAdapter, err := casbinInfra.NewCasbinAdapter(db, modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin adapter: %w", err)
	}

	return &authzInfrastructureComponents{
		casbinRuntime:           casbinInfra.NewRuntimeAdapters(casbinAdapter),
		roleRepository:          roleInfra.NewRoleRepository(db),
		bindingRepository:       bindingInfra.NewBindingRepository(db),
		resourceRepository:      resourceInfra.NewResourceRepository(db),
		policyVersionRepository: policyInfra.NewPolicyVersionRepository(db),
		permissionFactReader:    casbinrulerepo.NewFactReader(db),
		userRepository:          userInfra.NewRepository(db),
		unitOfWork:              mysqlAuthzUow.NewUnitOfWork(db, eventStager),
	}, nil
}
