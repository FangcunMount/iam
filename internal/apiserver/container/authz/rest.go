package authz

import (
	resttransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest"
	authzhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/authz/handler"
)

// CollectREST wires authz REST handlers when the module is available.
func CollectREST(available bool, mod *AuthzModule, deps *resttransport.Deps) {
	if !available || mod == nil || deps == nil {
		return
	}
	caps := mod.ApplicationCapabilities()
	deps.Authz.RoleHandler = authzhandler.NewRoleHandler(caps.RoleCatalog, caps.RoleDirectory)
	deps.Authz.RoleBindingHandler = authzhandler.NewRoleBindingHandler(caps.RoleBindingCommands, caps.RoleBindingDirectory)
	deps.Authz.PolicyHandler = authzhandler.NewPolicyHandler(caps.PermissionCommands, caps.PermissionReader, caps.PolicyLinter)
	deps.Authz.ResourceHandler = authzhandler.NewResourceHandler(caps.ResourceCatalog, caps.ResourceDirectory)
	deps.Authz.CheckHandler = authzhandler.NewCheckHandler(caps.AuthorizationChecker)
	deps.Authz.RouteAuthorization = caps.RouteAuthorization
	deps.Authz.HealthReporter = caps.RuntimeHealth
}
