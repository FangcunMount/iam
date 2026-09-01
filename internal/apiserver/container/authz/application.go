package authz

import (
	authorizationApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	permissionGrantApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/permissiongrant"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	bindingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/rolebinding"
	roleInheritanceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/roleinheritance"
)

func (m *AuthzModule) initializeApplication(
	infra *authzInfrastructureComponents,
	domain *authzDomainComponents,
) {
	m.resourceCatalog = resourceApp.NewResourceCatalog(domain.resourceValidator, infra.unitOfWork, infra.nativeRuntime)
	m.resourceDirectory = resourceApp.NewResourceQueryService(infra.resourceRepository)

	m.roleCatalog = roleApp.NewRoleCatalog(domain.roleValidator, infra.unitOfWork, infra.nativeRuntime)
	m.roleDirectory = roleApp.NewRoleQueryService(infra.roleRepository)

	m.permissionGrantService = permissionGrantApp.NewService(
		infra.unitOfWork,
		infra.permissionGrantRepository,
		infra.nativeRuntime,
	)
	m.roleInheritanceService = roleInheritanceApp.NewService(
		infra.unitOfWork,
		infra.roleInheritanceRepository,
		infra.nativeRuntime,
	)

	m.roleBindingCommands = bindingApp.NewCommandService(
		domain.roleBindingValidator,
		infra.roleRepository,
		infra.unitOfWork,
		infra.nativeRuntime,
	)
	m.roleBindingDirectory = bindingApp.NewDirectory(domain.roleBindingValidator, infra.bindingRepository)

	m.authorizationChecker = authorizationApp.NewChecker(infra.nativeRuntime)
	m.authorizationSnapshotReader = authorizationApp.NewSnapshotReader(infra.nativeRuntime)
}
