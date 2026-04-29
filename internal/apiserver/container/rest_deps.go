package container

import resttransport "github.com/FangcunMount/iam/internal/apiserver/transport/rest"

// BuildRESTDeps exposes only the collaborators required by the REST transport.
func (c *Container) BuildRESTDeps(options resttransport.RouterOptions) resttransport.Deps {
	deps := resttransport.Deps{
		RouterOptions: options,
	}
	if c == nil {
		return deps
	}

	deps.CacheGovernance = c.CacheGovernanceService
	deps.ModuleStatus.ContainerInitialized = true
	c.collectAuthnRESTDeps(&deps)
	c.collectAuthzRESTDeps(&deps)
	c.collectIDPRESTDeps(&deps)
	c.collectIdentityRESTDeps(&deps)
	c.collectSuggestRESTDeps(&deps)
	return deps
}

func (c *Container) collectAuthnRESTDeps(deps *resttransport.Deps) {
	if c.AuthnModule != nil {
		deps.ModuleStatus.Authn = true
		deps.Authn.AuthHandler = c.AuthnModule.AuthHandler
		deps.Authn.AccountHandler = c.AuthnModule.AccountHandler
		deps.Authn.JWKSHandler = c.AuthnModule.JWKSHandler
		deps.Authn.SessionAdminHandler = c.AuthnModule.SessionAdminHandler
		deps.Authn.TokenService = c.AuthnModule.TokenService
		deps.ModuleStatus.AuthEnabled = c.AuthnModule.TokenService != nil
	}
}

func (c *Container) collectAuthzRESTDeps(deps *resttransport.Deps) {
	if c.AuthzModule != nil {
		deps.ModuleStatus.Authz = true
		deps.Authz.RoleHandler = c.AuthzModule.RoleHandler
		deps.Authz.AssignmentHandler = c.AuthzModule.AssignmentHandler
		deps.Authz.PolicyHandler = c.AuthzModule.PolicyHandler
		deps.Authz.ResourceHandler = c.AuthzModule.ResourceHandler
		deps.Authz.CheckHandler = c.AuthzModule.CheckHandler
		deps.Authz.Casbin = c.AuthzModule.CasbinAdapter
		if reporter, ok := c.AuthzModule.CasbinAdapter.(resttransport.AuthzHealthReporter); ok {
			deps.Authz.HealthReporter = reporter
		}
	}
}

func (c *Container) collectIDPRESTDeps(deps *resttransport.Deps) {
	if c.IDPModule != nil {
		deps.ModuleStatus.IDP = true
		deps.IDP.WechatAppHandler = c.IDPModule.WechatAppHandler
	}
}

func (c *Container) collectIdentityRESTDeps(deps *resttransport.Deps) {
	if c.UserModule != nil {
		deps.ModuleStatus.User = true
		deps.User.UserHandler = c.UserModule.UserHandler
		deps.User.ChildHandler = c.UserModule.ChildHandler
		deps.User.GuardianshipHandler = c.UserModule.GuardianshipHandler
	}
}

func (c *Container) collectSuggestRESTDeps(deps *resttransport.Deps) {
	if c.SuggestModule != nil {
		deps.ModuleStatus.Suggest = c.SuggestModule.Service != nil
		deps.Suggest.Service = c.SuggestModule.Service
	}
}
