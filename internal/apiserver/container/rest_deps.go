package container

import (
	resttransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest"
	authnhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authn/handler"
	authzhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/handler"
	identityhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/handler"
	idphandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/idp/handler"
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
	deps.ModuleStatus.Container = toRESTModuleState(c.ContainerState())
	deps.ModuleStatus.ContainerInitialized = deps.ModuleStatus.Container.Bootstrapped
	deps.ModuleStatus.Modules = toRESTModuleStates(c.ModuleStates())
	c.collectAuthnRESTDeps(&deps)
	c.collectAuthzRESTDeps(&deps)
	c.collectIDPRESTDeps(&deps)
	c.collectIdentityRESTDeps(&deps)
	c.collectSuggestRESTDeps(&deps)
	return deps
}

func toRESTModuleStates(states map[string]ModuleState) map[string]resttransport.ModuleState {
	if len(states) == 0 {
		return nil
	}
	out := make(map[string]resttransport.ModuleState, len(states))
	for name, state := range states {
		out[name] = toRESTModuleState(state)
	}
	return out
}

func toRESTModuleState(state ModuleState) resttransport.ModuleState {
	return resttransport.ModuleState{
		Bootstrapped:   state.Bootstrapped,
		Available:      state.Available,
		DegradedReason: state.DegradedReason,
	}
}

func (c *Container) collectAuthnRESTDeps(deps *resttransport.Deps) {
	if c.ModuleState(moduleAuthn).Available {
		caps := c.AuthnModule.ApplicationCapabilities()
		deps.ModuleStatus.Authn = deps.ModuleStatus.Modules[moduleAuthn].Available
		deps.Authn.AuthHandler = authnhandler.NewAuthHandler(caps.LoginService, caps.TokenService, caps.LoginPreparationService)
		deps.Authn.OnboardingHandler = authnhandler.NewOnboardingHandler(caps.LoginIdentityOnboarder)
		deps.Authn.LoginIdentityOnboarder = caps.LoginIdentityOnboarder
		deps.Authn.JWKSHandler = authnhandler.NewJWKSHandler(caps.KeyManagementApp, caps.KeyPublishApp)
		deps.Authn.SessionAdminHandler = authnhandler.NewSessionAdminHandler(caps.SessionService)
		deps.Authn.TokenService = caps.TokenService
		deps.ModuleStatus.AuthEnabled = caps.TokenService != nil
	}
}

func (c *Container) collectAuthzRESTDeps(deps *resttransport.Deps) {
	if c.ModuleState(moduleAuthz).Available {
		caps := c.AuthzModule.ApplicationCapabilities()
		deps.ModuleStatus.Authz = deps.ModuleStatus.Modules[moduleAuthz].Available
		deps.Authz.RoleHandler = authzhandler.NewRoleHandler(caps.RoleCatalog, caps.RoleDirectory)
		deps.Authz.RoleBindingHandler = authzhandler.NewRoleBindingHandler(caps.RoleBindingCommands, caps.RoleBindingDirectory)
		deps.Authz.PolicyHandler = authzhandler.NewPolicyHandler(caps.PermissionCommands, caps.PermissionReader)
		deps.Authz.ResourceHandler = authzhandler.NewResourceHandler(caps.ResourceCatalog, caps.ResourceDirectory)
		deps.Authz.CheckHandler = authzhandler.NewCheckHandler(caps.AuthorizationChecker)
		deps.Authz.RouteAuthorization = caps.RouteAuthorization
		deps.Authz.HealthReporter = caps.RuntimeHealth
	}
}

func (c *Container) collectIDPRESTDeps(deps *resttransport.Deps) {
	if c.ModuleState(moduleIDP).Available {
		caps := c.IDPModule.ApplicationCapabilities()
		deps.ModuleStatus.IDP = deps.ModuleStatus.Modules[moduleIDP].Available
		deps.IDP.WechatAppHandler = idphandler.NewWechatAppHandler(
			caps.WechatAppService,
			caps.WechatAppCredentialService,
			caps.WechatAppTokenService,
		)
	}
}

func (c *Container) collectIdentityRESTDeps(deps *resttransport.Deps) {
	if c.ModuleState(moduleUser).Available {
		caps := c.UserModule.ApplicationCapabilities()
		deps.ModuleStatus.User = deps.ModuleStatus.Modules[moduleUser].Available
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
	if c.ModuleState(moduleSuggest).Available {
		caps := c.SuggestModule.ApplicationCapabilities()
		deps.ModuleStatus.Suggest = deps.ModuleStatus.Modules[moduleSuggest].Available
		deps.Suggest.Service = caps.Service
	}
}
