package authz

import (
	assignmentApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/assignment"
	authorizationApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/authorization"
	permissionGrantApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/permissiongrant"
	resourceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/resource"
	roleApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/role"
	roleInheritanceApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authz/roleinheritance"
)

func (m *AuthzModule) initializeApplication(
	infra *authzInfrastructureComponents,
	domain *authzDomainComponents,
) {
	m.authorizationDecisions = authorizationApp.NewDecisionService(infra.authorizationRuntime)
	m.resourceCatalog = resourceApp.NewResourceCatalog(infra.unitOfWork, infra.authorizationRuntime, m.authorizationDecisions)
	m.resourceDirectory = resourceApp.NewResourceQueryService(infra.resourceRepository)

	m.roleCatalog = roleApp.NewRoleCatalog(infra.unitOfWork, infra.authorizationRuntime)
	m.roleDirectory = roleApp.NewRoleQueryService(infra.roleRepository)

	m.permissionGrantService = permissionGrantApp.NewService(
		infra.unitOfWork,
		infra.permissionGrantRepository,
		infra.authorizationRuntime,
	)
	m.roleInheritanceService = roleInheritanceApp.NewService(
		infra.unitOfWork,
		infra.roleInheritanceRepository,
		infra.authorizationRuntime,
	)

	m.assignmentCommands = assignmentApp.NewCommandService(
		domain.assignmentValidator,
		infra.roleRepository,
		infra.unitOfWork,
		infra.authorizationRuntime,
	)
	m.assignmentDirectory = assignmentApp.NewDirectory(domain.assignmentValidator, infra.assignmentRepository)

	m.authorizationDecisions = authorizationApp.NewDecisionService(infra.authorizationRuntime)
	m.routeDecisionService = authorizationApp.NewRouteDecisionService(m.authorizationDecisions)
	m.authorizationSnapshotReader = authorizationApp.NewSnapshotReader(infra.authorizationRuntime)
}
