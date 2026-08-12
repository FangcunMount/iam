package authz

import (
	grpctransport "github.com/FangcunMount/iam/v3/internal/apiserver/transport/grpc"
	authzgrpc "github.com/FangcunMount/iam/v3/internal/apiserver/transport/grpc/service/authz"
)

// CollectGRPC appends authz gRPC registration when the module is available.
func CollectGRPC(available bool, mod *AuthzModule, registrations *[]grpctransport.Registration) {
	if !available || mod == nil || registrations == nil {
		return
	}
	caps := mod.ApplicationCapabilities()
	service := authzgrpc.NewService(
		caps.AuthorizationChecker,
		caps.AuthorizationSnapshotReader,
		caps.RoleBindingCommands,
		caps.AssignmentRequestAuthorizer,
	)
	*registrations = append(*registrations, grpctransport.Registration{
		Module:      "authz",
		Description: "AuthorizationService",
		Register:    service.Register,
	})
}
