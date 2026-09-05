package authz

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/subjectresolver"
	"gorm.io/gorm"

	authzuow "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/uow"
	assignmentDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/assignment"
	permissionGrantDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	policyDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	resourceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	roleDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/role"
	roleInheritanceDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/roleinheritance"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	authzRuntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
	assignmentInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/assignment"
	permissionGrantInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/permissiongrant"
	policyInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/policy"
	resourceInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/resource"
	roleInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/role"
	roleInheritanceInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/roleinheritance"
	mysqlAuthzUow "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/uow/authz"
	"github.com/FangcunMount/iam/v3/pkg/event"
)

type authzInfrastructureComponents struct {
	subjectResolver      assignmentDomain.SubjectResolver
	authorizationRuntime *authzRuntime.Runtime
	policySource         authzRuntime.Source

	roleRepository            roleDomain.Repository
	assignmentRepository      assignmentDomain.Repository
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
) *authzInfrastructureComponents {
	resolver := assignmentDomain.NewSubjectResolverRegistry(subjectresolver.NewUserSubjectResolver(userResolver))
	return &authzInfrastructureComponents{
		subjectResolver:           resolver,
		policySource:              authzRuntime.NewMySQLSource(db),
		roleRepository:            roleInfra.NewRoleRepository(db),
		assignmentRepository:      assignmentInfra.NewRepository(db),
		resourceRepository:        resourceInfra.NewResourceRepository(db),
		policyVersionRepository:   policyInfra.NewPolicyVersionRepository(db),
		permissionGrantRepository: permissionGrantInfra.NewRepository(db),
		roleInheritanceRepository: roleInheritanceInfra.NewRepository(db),
		unitOfWork:                mysqlAuthzUow.NewUnitOfWork(db, resolver, eventStager),
	}
}
