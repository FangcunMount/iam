package container

import (
	"github.com/FangcunMount/component-base/pkg/log"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	messagingInfra "github.com/FangcunMount/iam/internal/apiserver/infra/messaging"
	"github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

type moduleGraph struct {
	container *Container
}

func (c *Container) moduleGraph() *moduleGraph {
	return &moduleGraph{container: c}
}

func (g *moduleGraph) policyVersionNotifier() policyDomain.VersionNotifier {
	if g == nil || g.container == nil || g.container.eventBus == nil {
		log.Warn("   ⚠️  Policy version notifier: disabled (no EventBus)")
		return nil
	}
	log.Info("   📨 Policy version notifier: NSQ EventBus")
	return messagingInfra.NewVersionNotifier(g.container.eventBus)
}

func (g *moduleGraph) userModuleDependencies() []interface{} {
	deps := make([]interface{}, 0, 2)
	if casbin := g.casbinEnforcer(); casbin != nil {
		deps = append(deps, casbin)
	}
	if sessionManager := g.sessionManager(); sessionManager != nil {
		deps = append(deps, sessionManager)
	}
	return deps
}

func (g *moduleGraph) casbinEnforcer() authn.CasbinEnforcer {
	if g == nil || g.container == nil || g.container.AuthzModule == nil {
		return nil
	}
	return g.container.AuthzModule.CasbinAdapter
}

func (g *moduleGraph) sessionManager() interface{} {
	if g == nil || g.container == nil || g.container.AuthnModule == nil {
		return nil
	}
	return g.container.AuthnModule.SessionManager()
}
