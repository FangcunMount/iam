package grpc

import (
	"github.com/FangcunMount/component-base/pkg/log"
	grpcpkg "github.com/FangcunMount/iam/v3/internal/pkg/grpc"
	googlegrpc "google.golang.org/grpc"
)

// Registration is a transport-owned gRPC service registration entry.
type Registration struct {
	Module      string
	Description string
	Register    func(*googlegrpc.Server)
}

// Deps is the dependency surface required to register IAM gRPC services.
type Deps struct {
	Server        *grpcpkg.Server
	Registrations []Registration
}

// Registry owns gRPC service registration for the apiserver transport.
type Registry struct {
	server        *grpcpkg.Server
	registrations []Registration
}

func NewRegistry(deps Deps) *Registry {
	return &Registry{
		server:        deps.Server,
		registrations: deps.Registrations,
	}
}

// RegisterServices registers all configured module services in startup order.
func (r *Registry) RegisterServices() error {
	if r == nil || r.server == nil {
		log.Warn("gRPC server is nil, skipping service registration")
		return nil
	}

	for _, registration := range r.registrations {
		if registration.Register == nil {
			continue
		}
		registration.Register(r.server.Server)
		log.Infow("📡 Registered gRPC services",
			"module", registration.Module,
			"description", registration.Description,
		)
	}

	log.Info("✅ All gRPC services registered successfully")
	r.server.MarkAllServicesServing()
	return nil
}
