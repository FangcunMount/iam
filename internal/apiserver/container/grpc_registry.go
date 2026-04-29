package container

import googlegrpc "google.golang.org/grpc"

// GRPCRegistration is a transport-facing registration entry exposed by the
// container without requiring the apiserver to inspect concrete module fields.
type GRPCRegistration struct {
	Module      string
	Description string
	Register    func(*googlegrpc.Server)
}

// GRPCRegistrations returns gRPC module registrars in the server startup order.
func (c *Container) GRPCRegistrations() []GRPCRegistration {
	if c == nil {
		return nil
	}

	registrations := make([]GRPCRegistration, 0, 4)
	if c.AuthnModule != nil && c.AuthnModule.GRPCService != nil {
		service := c.AuthnModule.GRPCService
		registrations = append(registrations, GRPCRegistration{
			Module:      "authn",
			Description: "AuthService, JWKSService",
			Register:    service.Register,
		})
	}
	if c.UserModule != nil && c.UserModule.GRPCService != nil {
		service := c.UserModule.GRPCService
		registrations = append(registrations, GRPCRegistration{
			Module:      "user",
			Description: "IdentityRead, GuardianshipQuery, GuardianshipCommand, IdentityLifecycle",
			Register:    service.Register,
		})
	}
	if c.IDPModule != nil && c.IDPModule.GRPCService != nil {
		service := c.IDPModule.GRPCService
		registrations = append(registrations, GRPCRegistration{
			Module:      "idp",
			Description: "IDPService",
			Register:    service.Register,
		})
	}
	if c.AuthzModule != nil && c.AuthzModule.GRPCService != nil {
		service := c.AuthzModule.GRPCService
		registrations = append(registrations, GRPCRegistration{
			Module:      "authz",
			Description: "AuthorizationService",
			Register:    service.Register,
		})
	}
	return registrations
}
