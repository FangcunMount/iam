package container

import (
	externalidentity "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
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
		EffectiveRoles: g.effectiveRoleReader(),
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
		ExternalIdentity: externalidentity.Config{
			WeComAgentID: g.container.runtimeOptions.IDP.WeCom.AgentID,
		},
	}
}

func (g *moduleGraph) authnModuleDependencies() authn.AuthnModuleDeps {
	userAccess := g.identityUserAccessCapabilities()
	return authn.AuthnModuleDeps{
		DB:               g.container.mysqlDB,
		RedisClient:      g.container.redisClient,
		IDPModule:        g.container.IDPModule,
		EventPublisher:   g.container.eventPublisher,
		Environment:      g.container.runtimeOptions.Environment,
		Auth:             g.container.runtimeOptions.Auth,
		JWKS:             g.container.runtimeOptions.JWKS,
		WechatOpen:       g.container.runtimeOptions.IDP.WechatOpen,
		SMS:              g.container.runtimeOptions.SMS,
		UserStatusReader: userAccess.UserStatusReader,
	}
}

func (g *moduleGraph) authzModuleDependencies() authz.AuthzModuleDeps {
	userAccess := g.identityUserAccessCapabilities()
	return authz.AuthzModuleDeps{
		SyncConfig:                g.container.runtimeOptions.Authz.PolicySync,
		AttributeProvidersFile:    g.container.runtimeOptions.Authz.AttributeProvidersFile,
		DB:                        g.container.mysqlDB,
		EventStager:               g.container.outboxStore,
		GRPCACLEnabled:            g.container.runtimeOptions.GRPCACLEnabled,
		GRPCACLConfigFile:         g.container.runtimeOptions.GRPCACLConfigFile,
		AssignmentConstraintsFile: g.container.runtimeOptions.GRPCAssignmentConstraintsFile,
		UserResolver:              userAccess.UserResolver,
	}
}

func (g *moduleGraph) identityUserAccessCapabilities() identity.UserAccessCapabilities {
	if g == nil || g.container == nil {
		return identity.UserAccessCapabilities{}
	}
	capabilities := g.container.identityUserAccess
	if capabilities.UserStatusReader == nil || capabilities.UserResolver == nil {
		capabilities = identity.NewUserAccessCapabilities(g.container.mysqlDB)
		g.container.identityUserAccess = capabilities
	}
	return capabilities
}

func (g *moduleGraph) suggestModuleDependencies() suggest.SuggestModuleDeps {
	deps := suggest.SuggestModuleDeps{
		DB:          g.container.mysqlDB,
		Config:      g.container.runtimeOptions.Suggest,
		Environment: g.container.runtimeOptions.Environment,
		RedisClient: g.container.redisClient,
	}
	if g.container.AuthzModule != nil {
		deps.RoutePermissionChecker = g.container.AuthzModule.ApplicationCapabilities().RoutePermissionChecker
	}
	return deps
}

func (g *moduleGraph) effectiveRoleReader() identity.EffectiveRoleReader {
	if g == nil || g.container == nil || g.container.AuthzModule == nil {
		return nil
	}
	return g.container.AuthzModule.EffectiveRoleReader()
}

func (g *moduleGraph) sessionRevoker() sessiondomain.Revoker {
	if g == nil || g.container == nil || g.container.AuthnModule == nil {
		return nil
	}
	return g.container.AuthnModule.SessionRevoker()
}
