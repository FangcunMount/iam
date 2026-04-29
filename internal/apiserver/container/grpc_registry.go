package container

import (
	grpctransport "github.com/FangcunMount/iam/internal/apiserver/transport/grpc"
	grpcpkg "github.com/FangcunMount/iam/internal/pkg/grpc"
)

// BuildGRPCDeps exposes only the collaborators required by the gRPC transport.
func (c *Container) BuildGRPCDeps(server *grpcpkg.Server) grpctransport.Deps {
	deps := grpctransport.Deps{Server: server}
	if c == nil {
		return deps
	}

	registrations := make([]grpctransport.Registration, 0, 4)
	if c.AuthnModule != nil && c.AuthnModule.GRPCService != nil {
		service := c.AuthnModule.GRPCService
		registrations = append(registrations, grpctransport.Registration{
			Module:      "authn",
			Description: "AuthService, JWKSService",
			Register:    service.Register,
		})
	}
	if c.UserModule != nil && c.UserModule.GRPCService != nil {
		service := c.UserModule.GRPCService
		registrations = append(registrations, grpctransport.Registration{
			Module:      "user",
			Description: "IdentityRead, GuardianshipQuery, GuardianshipCommand, IdentityLifecycle",
			Register:    service.Register,
		})
	}
	if c.IDPModule != nil && c.IDPModule.GRPCService != nil {
		service := c.IDPModule.GRPCService
		registrations = append(registrations, grpctransport.Registration{
			Module:      "idp",
			Description: "IDPService",
			Register:    service.Register,
		})
	}
	if c.AuthzModule != nil && c.AuthzModule.GRPCService != nil {
		service := c.AuthzModule.GRPCService
		registrations = append(registrations, grpctransport.Registration{
			Module:      "authz",
			Description: "AuthorizationService",
			Register:    service.Register,
		})
	}
	deps.Registrations = registrations
	return deps
}
