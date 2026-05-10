package assembler

func (m *AuthzModule) initializeRuntime(infra *authzInfrastructureComponents) {
	m.routeAuthorization = infra.casbinAdapter
	m.roleNames = infra.casbinAdapter
	m.runtimeHealth = infra.casbinAdapter
}
