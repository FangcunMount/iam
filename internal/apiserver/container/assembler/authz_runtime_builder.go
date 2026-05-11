package assembler

func (m *AuthzModule) initializeRuntime(infra *authzInfrastructureComponents) {
	m.routeAuthorization = infra.casbinRuntime.RouteAuthorizer
	m.roleNames = infra.casbinRuntime.RoleNameReader
	m.runtimeHealth = infra.casbinRuntime.RuntimeHealthReporter
	m.policyReloader = infra.casbinRuntime.PolicyReloader
}
