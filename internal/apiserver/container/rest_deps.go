package container

import (
	resttransport "github.com/FangcunMount/iam/internal/apiserver/transport/rest"
	authnhandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authn/handler"
	authzhandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/authz/handler"
	identityhandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/identity/handler"
	idphandler "github.com/FangcunMount/iam/internal/apiserver/transport/rest/idp/handler"
)

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
		caps := c.AuthnModule.ApplicationCapabilities()
		deps.ModuleStatus.Authn = true
		deps.Authn.AuthHandler = authnhandler.NewAuthHandler(caps.LoginService, caps.TokenService, caps.LoginPreparationService)
		deps.Authn.AccountHandler = authnhandler.NewAccountHandlerWithRoles(
			caps.AccountService,
			caps.AccountService,
			caps.AccountService,
			caps.AccountOnboarder,
		)
		deps.Authn.JWKSHandler = authnhandler.NewJWKSHandler(caps.KeyManagementApp, caps.KeyPublishApp)
		deps.Authn.SessionAdminHandler = authnhandler.NewSessionAdminHandler(caps.SessionService)
		deps.Authn.TokenService = caps.TokenService
		deps.ModuleStatus.AuthEnabled = caps.TokenService != nil
	}
}

func (c *Container) collectAuthzRESTDeps(deps *resttransport.Deps) {
	if c.AuthzModule != nil {
		caps := c.AuthzModule.ApplicationCapabilities()
		deps.ModuleStatus.Authz = true
		deps.Authz.RoleHandler = authzhandler.NewRoleHandler(caps.RoleCatalog, caps.RoleDirectory)
		deps.Authz.RoleBindingHandler = authzhandler.NewRoleBindingHandler(caps.RoleBindingCommands, caps.RoleBindingDirectory)
		deps.Authz.PolicyHandler = authzhandler.NewPolicyHandler(caps.PermissionCommands, caps.PermissionReader)
		deps.Authz.ResourceHandler = authzhandler.NewResourceHandler(caps.ResourceCatalog, caps.ResourceDirectory)
		deps.Authz.CheckHandler = authzhandler.NewCheckHandler(caps.AuthorizationChecker)
		deps.Authz.RouteAuthorization = caps.RouteAuthorization
		if reporter, ok := caps.RouteAuthorization.(resttransport.AuthzHealthReporter); ok {
			deps.Authz.HealthReporter = reporter
		}
	}
}

func (c *Container) collectIDPRESTDeps(deps *resttransport.Deps) {
	if c.IDPModule != nil {
		caps := c.IDPModule.ApplicationCapabilities()
		deps.ModuleStatus.IDP = true
		deps.IDP.WechatAppHandler = idphandler.NewWechatAppHandler(
			caps.WechatAppService,
			caps.WechatAppCredentialService,
			caps.WechatAppTokenService,
		)
	}
}

func (c *Container) collectIdentityRESTDeps(deps *resttransport.Deps) {
	if c.UserModule != nil {
		caps := c.UserModule.ApplicationCapabilities()
		deps.ModuleStatus.User = true
		deps.User.UserHandler = identityhandler.NewUserHandler(
			caps.UserCreator,
			caps.UserEditor,
			caps.UserDirectory,
			caps.RoleNames,
		)
		deps.User.ProfileHandler = identityhandler.NewProfileHandler(
			caps.MyProfiles,
			caps.ProfileDirectory,
		)
		deps.User.ProfileLinkHandler = identityhandler.NewProfileLinkHandler(caps.MyProfileLinks)
	}
}

func (c *Container) collectSuggestRESTDeps(deps *resttransport.Deps) {
	if c.SuggestModule != nil {
		caps := c.SuggestModule.ApplicationCapabilities()
		deps.ModuleStatus.Suggest = caps.Service != nil
		deps.Suggest.Service = caps.Service
	}
}
