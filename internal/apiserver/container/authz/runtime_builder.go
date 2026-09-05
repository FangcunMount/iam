package authz

import (
	"context"
	"fmt"

	authzRuntime "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/runtime"
)

func (m *AuthzModule) initializeRuntime(infra *authzInfrastructureComponents, domain *authzDomainComponents, config authzRuntime.Config) error {
	if config == (authzRuntime.Config{}) {
		config = authzRuntime.DefaultConfig()
	}
	runtime, err := authzRuntime.NewRuntime(
		context.Background(),
		infra.policySource,
		domain.authorizationEvaluator, authzRuntime.WithConfig(config), authzRuntime.WithAttributeProviders(m.attributeProviders),
	)
	if err != nil {
		return fmt.Errorf("failed to create authorization snapshot runtime: %w", err)
	}
	runtime.RequireSync()
	infra.authorizationRuntime = runtime
	m.effectiveRoles = runtime
	m.runtimeHealth = runtime
	m.policyReloader = runtime
	return nil
}
