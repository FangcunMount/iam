package container

import (
	"github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

type moduleGraph struct {
	container *Container
}

func (c *Container) moduleGraph() *moduleGraph {
	return &moduleGraph{container: c}
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
