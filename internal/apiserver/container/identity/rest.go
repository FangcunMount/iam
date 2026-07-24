package identity

import (
	resttransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest"
	identityhandler "github.com/FangcunMount/iam/v2/internal/apiserver/transport/rest/identity/handler"
)

// CollectREST wires identity REST handlers when the module is available.
func CollectREST(available bool, mod *IdentityModule, moduleName string, deps *resttransport.Deps) {
	if !available || mod == nil || deps == nil {
		return
	}
	caps := mod.ApplicationCapabilities()
	deps.ModuleStatus.User = deps.ModuleStatus.Modules[moduleName].Available
	deps.User.UserHandler = identityhandler.NewUserHandler(
		caps.UserCreator,
		caps.UserEditor,
		caps.UserDirectory,
		caps.RoleNames,
	)
	deps.User.ProfileHandler = identityhandler.NewProfileHandler(caps.MyProfiles)
	deps.User.ProfileLinkHandler = identityhandler.NewProfileLinkHandler(caps.MyProfileLinks)
}
