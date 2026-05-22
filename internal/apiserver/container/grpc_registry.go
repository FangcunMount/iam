package container

import (
	grpctransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc"
	authngrpc "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc/service/authn"
	authzgrpc "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc/service/authz"
	identitygrpc "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc/service/identity"
	idpgrpc "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc/service/idp"
	grpcpkg "github.com/FangcunMount/iam/v2/internal/pkg/grpc"
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
	if c.ModuleState(moduleAuthn).Available {
		caps := c.AuthnModule.ApplicationCapabilities()
		service := authngrpc.NewService(
			caps.SessionService,
			caps.TokenService,
			caps.SignupService,
			caps.LoginPhoneOTPSender,
			caps.PhoneLinkOTPSender,
			caps.LoginIdentityLinking,
			caps.KeyPublishApp,
		)
		registrations = append(registrations, grpctransport.Registration{
			Module:      "authn",
			Description: "AuthService, AuthSignupService, AuthChallengeService, LoginIdentityService, JWKSService",
			Register:    service.Register,
		})
	}
	if c.ModuleState(moduleUser).Available {
		caps := c.UserModule.ApplicationCapabilities()
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
		registrations = append(registrations, grpctransport.Registration{
			Module:      "user",
			Description: "IdentityRead, ProfileLinkQuery, ProfileCommand, ProfileLinkCommand, IdentityLifecycle",
			Register:    service.Register,
		})
	}
	if c.ModuleState(moduleIDP).Available {
		caps := c.IDPModule.ApplicationCapabilities()
		service := idpgrpc.NewService(caps.WechatAppService, caps.WechatAppTokenService, caps.WechatAppRepository, caps.SecretVault)
		registrations = append(registrations, grpctransport.Registration{
			Module:      "idp",
			Description: "IDPService",
			Register:    service.Register,
		})
	}
	if c.ModuleState(moduleAuthz).Available {
		caps := c.AuthzModule.ApplicationCapabilities()
		service := authzgrpc.NewService(
			caps.AuthorizationChecker,
			caps.AuthorizationSnapshotReader,
			caps.RoleBindingCommands,
		)
		registrations = append(registrations, grpctransport.Registration{
			Module:      "authz",
			Description: "AuthorizationService",
			Register:    service.Register,
		})
	}
	return registrations
}
