package container

import (
	grpctransport "github.com/FangcunMount/iam/v2/internal/apiserver/transport/grpc"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authn"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/identity"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/idp"
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
	authn.CollectGRPC(c.ModuleState(moduleAuthn).Available, c.AuthnModule, &registrations)
	identity.CollectGRPC(c.ModuleState(moduleIdentity).Available, c.IdentityModule, &registrations)
	idp.CollectGRPC(c.ModuleState(moduleIDP).Available, c.IDPModule, &registrations)
	authz.CollectGRPC(c.ModuleState(moduleAuthz).Available, c.AuthzModule, &registrations)
	return registrations
}
