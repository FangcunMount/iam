package authz

import (
	"context"
	"fmt"

	nativeInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/authz/native"
)

func (m *AuthzModule) initializeRuntime(infra *authzInfrastructureComponents, domain *authzDomainComponents) error {
	nativeRuntime, err := nativeInfra.NewRuntime(
		context.Background(),
		infra.nativeSource,
		domain.authorizationEvaluator,
	)
	if err != nil {
		return fmt.Errorf("failed to create native authorization runtime: %w", err)
	}
	infra.nativeRuntime = nativeRuntime
	m.routeAuthorization = infra.nativeRuntime
	m.roleNames = infra.nativeRuntime
	m.runtimeHealth = infra.nativeRuntime
	m.policyReloader = infra.nativeRuntime
	return nil
}
