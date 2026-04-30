package container

import (
	"github.com/FangcunMount/iam/internal/apiserver/container/assembler"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
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
		RoleNames:      g.roleNameReader(),
		SessionManager: g.sessionManager(),
	}
}

func (g *moduleGraph) idpModuleDependencies() assembler.IDPModuleDeps {
	return assembler.IDPModuleDeps{
		DB:            g.container.mysqlDB,
		RedisClient:   g.container.redisClient,
		EncryptionKey: g.container.idpEncryptionKey,
	}
}

func (g *moduleGraph) authnModuleDependencies() assembler.AuthnModuleDeps {
	return assembler.AuthnModuleDeps{
		DB:             g.container.mysqlDB,
		RedisClient:    g.container.redisClient,
		IDPModule:      g.container.IDPModule,
		EventBus:       g.container.eventBus,
		EventPublisher: g.container.eventPublisher,
		AppMode:        g.container.runtimeOptions.AppMode,
		Auth:           g.container.runtimeOptions.Auth,
		JWKS:           g.container.runtimeOptions.JWKS,
		IDPOptions:     g.container.runtimeOptions.IDP,
		SMS:            g.container.runtimeOptions.SMS,
	}
}

func (g *moduleGraph) authzModuleDependencies() assembler.AuthzModuleDeps {
	return assembler.AuthzModuleDeps{
		DB:          g.container.mysqlDB,
		EventStager: g.container.outboxStore,
	}
}

func (g *moduleGraph) suggestModuleDependencies() assembler.SuggestModuleDeps {
	return assembler.SuggestModuleDeps{
		DB:     g.container.mysqlDB,
		Config: g.container.runtimeOptions.Suggest,
	}
}

func (g *moduleGraph) roleNameReader() assembler.RoleNameReader {
	if g == nil || g.container == nil || g.container.AuthzModule == nil {
		return nil
	}
	return g.container.AuthzModule.RoleNameReader()
}

func (g *moduleGraph) sessionManager() sessiondomain.Manager {
	if g == nil || g.container == nil || g.container.AuthnModule == nil {
		return nil
	}
	return g.container.AuthnModule.SessionManager()
}
