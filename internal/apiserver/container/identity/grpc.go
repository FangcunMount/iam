package identity

import (
	grpctransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc"
	identitygrpc "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc/service/identity"
)

// CollectGRPC appends identity gRPC registration when the module is available.
func CollectGRPC(available bool, mod *IdentityModule, registrations *[]grpctransport.Registration) {
	if !available || mod == nil || registrations == nil {
		return
	}
	caps := mod.ApplicationCapabilities()
	service := identitygrpc.NewService(
		caps.UserDirectory,
		caps.ProfileDirectory,
		caps.ProfileLinkDirectory,
		caps.UserCreator,
		caps.UserEditor,
		caps.UserStatusChanger,
		caps.MyProfiles,
		caps.ProfileLinkCommands,
		caps.MyProfileLinks,
	)
	*registrations = append(*registrations, grpctransport.Registration{
		Module:      "identity",
		Description: "IdentityRead, ProfileLinkQuery, ProfileCommand, ProfileLinkCommand, IdentityLifecycle",
		Register:    service.Register,
	})
}
