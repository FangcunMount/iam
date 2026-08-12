package container

import (
	"github.com/FangcunMount/iam/v3/internal/apiserver/container/authn"
	"github.com/FangcunMount/iam/v3/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v3/internal/apiserver/container/identity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/container/idp"
	"github.com/FangcunMount/iam/v3/internal/apiserver/container/suggest"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	sessionrevocation "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/sessionrevocation"
)

type moduleGraph struct {
	container *Container
}

func (c *Container) moduleGraph() *moduleGraph {
	return &moduleGraph{container: c}
}

func (g *moduleGraph) identityModuleDependencies() identity.IdentityModuleDeps {
	revocation := g.container.runtimeOptions.Identity.SessionRevocation
	return identity.IdentityModuleDeps{
		DB:             g.container.mysqlDB,
		RoleNames:      g.roleNameReader(),
		SessionRevoker: g.sessionRevoker(),
		SessionRevocationConfig: sessionrevocation.WorkerConfig{
			PollInterval:         revocation.PollInterval,
			BatchSize:            revocation.BatchSize,
			RetryBaseDelay:       revocation.RetryBaseDelay,
			RetryMaxDelay:        revocation.RetryMaxDelay,
			StaleProcessingAfter: revocation.StaleProcessingAfter,
		},
	}
}

func (g *moduleGraph) idpModuleDependencies() idp.IDPModuleDeps {
	return idp.IDPModuleDeps{
		DB:            g.container.mysqlDB,
		RedisClient:   g.container.redisClient,
		EncryptionKey: g.container.idpEncryptionKey,
	}
}

func (g *moduleGraph) authnModuleDependencies() authn.AuthnModuleDeps {
	return authn.AuthnModuleDeps{
		DB:             g.container.mysqlDB,
		RedisClient:    g.container.redisClient,
		IDPModule:      g.container.IDPModule,
		EventPublisher: g.container.eventPublisher,
		Environment:    g.container.runtimeOptions.Environment,
		Auth:           g.container.runtimeOptions.Auth,
		JWKS:           g.container.runtimeOptions.JWKS,
		IDPOptions:     g.container.runtimeOptions.IDP,
		SMS:            g.container.runtimeOptions.SMS,
	}
}

func (g *moduleGraph) authzModuleDependencies() authz.AuthzModuleDeps {
	return authz.AuthzModuleDeps{
		DB:                        g.container.mysqlDB,
		EventStager:               g.container.outboxStore,
		GRPCACLEnabled:            g.container.runtimeOptions.GRPCACLEnabled,
		GRPCACLConfigFile:         g.container.runtimeOptions.GRPCACLConfigFile,
		AssignmentConstraintsFile: g.container.runtimeOptions.GRPCAssignmentConstraintsFile,
	}
}

func (g *moduleGraph) suggestModuleDependencies() suggest.SuggestModuleDeps {
	deps := suggest.SuggestModuleDeps{
		DB:          g.container.mysqlDB,
		Config:      g.container.runtimeOptions.Suggest,
		Environment: g.container.runtimeOptions.Environment,
		RedisClient: g.container.redisClient,
	}
	if g.container.AuthzModule != nil {
		deps.RouteAuthorization = g.container.AuthzModule.ApplicationCapabilities().RouteAuthorization
	}
	return deps
}

func (g *moduleGraph) roleNameReader() identity.RoleNameReader {
	if g == nil || g.container == nil || g.container.AuthzModule == nil {
		return nil
	}
	return g.container.AuthzModule.RoleNameReader()
}

func (g *moduleGraph) sessionRevoker() sessiondomain.Revoker {
	if g == nil || g.container == nil || g.container.AuthnModule == nil {
		return nil
	}
	return g.container.AuthnModule.SessionRevoker()
}
