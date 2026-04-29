package container

import (
	"github.com/FangcunMount/iam/internal/apiserver/container/assembler"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/pkg/middleware/authn"
)

type moduleGraph struct {
	container *Container
}

func (c *Container) moduleGraph() *moduleGraph {
	return &moduleGraph{container: c}
}

func (g *moduleGraph) userModuleDependencies() assembler.UserModuleDeps {
	return assembler.UserModuleDeps{
		DB:             g.container.mysqlDB,
		Casbin:         g.casbinEnforcer(),
		SessionManager: g.sessionManager(),
	}
}

func (g *moduleGraph) casbinEnforcer() authn.CasbinEnforcer {
	if g == nil || g.container == nil || g.container.AuthzModule == nil {
		return nil
	}
	return g.container.AuthzModule.CasbinAdapter
}

func (g *moduleGraph) sessionManager() sessiondomain.Manager {
	if g == nil || g.container == nil || g.container.AuthnModule == nil {
		return nil
	}
	return g.container.AuthnModule.SessionManager()
}
