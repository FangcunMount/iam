package assembler

import (
	authorizationApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/authorization"
	policyApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/policy"
	resourceApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/role"
	bindingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authz/rolebinding"
)

func (m *AuthzModule) initializeApplication(
	infra *authzInfrastructureComponents,
	domain *authzDomainComponents,
) {
	m.resourceCatalog = resourceApp.NewResourceCatalog(domain.resourceValidator, infra.resourceRepository)
	m.resourceDirectory = resourceApp.NewResourceQueryService(infra.resourceRepository)

	m.roleCatalog = roleApp.NewRoleCatalog(domain.roleValidator, infra.roleRepository)
	m.roleDirectory = roleApp.NewRoleQueryService(infra.roleRepository)

	m.permissionCommands = policyApp.NewPolicyCommandService(domain.policyValidator, infra.unitOfWork, infra.casbinAdapter)
	m.permissionReader = policyApp.NewPolicyQueryService(
		infra.policyVersionRepository,
		infra.casbinAdapter,
		infra.roleRepository,
	)

	m.roleBindingCommands = bindingApp.NewCommandService(
		domain.roleBindingValidator,
		infra.roleRepository,
		infra.unitOfWork,
		infra.casbinAdapter,
	)
	m.roleBindingDirectory = bindingApp.NewDirectory(domain.roleBindingValidator, infra.bindingRepository)

	m.authorizationChecker = authorizationApp.NewChecker(infra.casbinAdapter)
	m.authorizationSnapshotReader = authorizationApp.NewSnapshotReader(infra.casbinAdapter, infra.policyVersionRepository)
}
