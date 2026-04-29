package container

import (
	grpctransport "github.com/FangcunMount/iam/internal/apiserver/transport/grpc"
	authngrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/authn"
	authzgrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/authz"
	idpgrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/idp"
	ucgrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/uc"
	identitygrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/uc/identity"
	grpcpkg "github.com/FangcunMount/iam/internal/pkg/grpc"
)

// BuildGRPCDeps exposes only the collaborators required by the gRPC transport.
func (c *Container) BuildGRPCDeps(server *grpcpkg.Server) grpctransport.Deps {
	deps := grpctransport.Deps{Server: server}
	if c == nil {
		return deps
	}

	deps.Registrations = c.grpcRegistrations()
	return deps
}

func (c *Container) grpcRegistrations() []grpctransport.Registration {
	registrations := make([]grpctransport.Registration, 0, 4)
	if c.AuthnModule != nil {
		caps := c.AuthnModule.ApplicationCapabilities()
		service := authngrpc.NewService(caps.TokenService, caps.RegisterService, caps.KeyPublishApp)
		registrations = append(registrations, grpctransport.Registration{
			Module:      "authn",
			Description: "AuthService, JWKSService",
			Register:    service.Register,
		})
	}
	if c.UserModule != nil {
		caps := c.UserModule.ApplicationCapabilities()
		identitySvc := identitygrpc.NewService(
			caps.UserQueryService,
			caps.ProfileQueryService,
			caps.ProfileLinkQueryService,
			caps.UserService,
			caps.UserProfileService,
			caps.UserStatusService,
			caps.ProfileLinkService,
			caps.ProfileLinkAccessService,
		)
		service := ucgrpc.NewService(identitySvc)
		registrations = append(registrations, grpctransport.Registration{
			Module:      "user",
			Description: "IdentityRead, ProfileLinkQuery, ProfileLinkCommand, IdentityLifecycle",
			Register:    service.Register,
		})
	}
	if c.IDPModule != nil {
		caps := c.IDPModule.ApplicationCapabilities()
		service := idpgrpc.NewService(caps.WechatAppService, caps.WechatAppRepository, caps.SecretVault)
		registrations = append(registrations, grpctransport.Registration{
			Module:      "idp",
			Description: "IDPService",
			Register:    service.Register,
		})
	}
	if c.AuthzModule != nil {
		caps := c.AuthzModule.ApplicationCapabilities()
		service := authzgrpc.NewService(caps.Casbin, caps.RoleRepository, caps.PolicyVersionRepo, caps.AssignmentCommander)
		registrations = append(registrations, grpctransport.Registration{
			Module:      "authz",
			Description: "AuthorizationService",
			Register:    service.Register,
		})
	}
	return registrations
}
