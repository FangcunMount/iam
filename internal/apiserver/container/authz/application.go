package authz

import (
	authorizationApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	policyApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policy"
	policylintApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/policylint"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	bindingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/rolebinding"
)

func (m *AuthzModule) initializeApplication(
	infra *authzInfrastructureComponents,
	domain *authzDomainComponents,
) {
	m.resourceCatalog = resourceApp.NewResourceCatalog(domain.resourceValidator, infra.resourceRepository)
	m.resourceDirectory = resourceApp.NewResourceQueryService(infra.resourceRepository)

	m.roleCatalog = roleApp.NewRoleCatalog(domain.roleValidator, infra.roleRepository)
	m.roleDirectory = roleApp.NewRoleQueryService(infra.roleRepository)

	m.permissionCommands = policyApp.NewPolicyCommandService(domain.policyValidator, infra.unitOfWork, infra.casbinRuntime.PolicyReloader)
	m.permissionReader = policyApp.NewPolicyQueryService(
		infra.policyVersionRepository,
		infra.casbinRuntime.RolePermissionStore,
		infra.roleRepository,
	)
	m.policyLinter = policylintApp.NewLinter(infra.permissionFactReader, infra.resourceRepository)

	m.roleBindingCommands = bindingApp.NewCommandService(
		domain.roleBindingValidator,
		infra.roleRepository,
		infra.unitOfWork,
		infra.casbinRuntime.PolicyReloader,
	)
	m.roleBindingDirectory = bindingApp.NewDirectory(domain.roleBindingValidator, infra.bindingRepository)

	m.authorizationChecker = authorizationApp.NewChecker(infra.casbinRuntime.DecisionEngine, infra.policyVersionRepository)
	m.authorizationSnapshotReader = authorizationApp.NewSnapshotReader(infra.casbinRuntime.SnapshotStore, infra.policyVersionRepository)
}
