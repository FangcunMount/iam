package authz

func (m *AuthzModule) initializeRuntime(infra *authzInfrastructureComponents) {
	m.routeAuthorization = infra.nativeRuntime
	m.roleNames = infra.nativeRuntime
	m.runtimeHealth = infra.nativeRuntime
	m.policyReloader = infra.nativeRuntime
}
