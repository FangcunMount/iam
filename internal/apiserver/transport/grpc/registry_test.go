package grpc

import (
	"reflect"
	"testing"

	grpcpkg "github.com/FangcunMount/iam/v2/internal/pkg/grpc"
	googlegrpc "google.golang.org/grpc"
)

func TestRegistryRegistersServicesInConfiguredOrder(t *testing.T) {
	order := []string{}
	server := &grpcpkg.Server{Server: googlegrpc.NewServer()}
	defer server.Stop()

	err := NewRegistry(Deps{
		Server: server,
		Registrations: []Registration{
			{Module: "authn", Description: "AuthService", Register: func(*googlegrpc.Server) { order = append(order, "authn") }},
			{Module: "user", Description: "IdentityRead", Register: func(*googlegrpc.Server) { order = append(order, "user") }},
			{Module: "idp", Description: "IDPService", Register: func(*googlegrpc.Server) { order = append(order, "idp") }},
			{Module: "authz", Description: "AuthorizationService", Register: func(*googlegrpc.Server) { order = append(order, "authz") }},
		},
	}).RegisterServices()
	if err != nil {
		t.Fatalf("RegisterServices() error = %v", err)
	}

	want := []string{"authn", "user", "idp", "authz"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("registration order = %#v, want %#v", order, want)
	}
}

func TestRegistryHandlesNilServer(t *testing.T) {
	if err := NewRegistry(Deps{}).RegisterServices(); err != nil {
		t.Fatalf("RegisterServices() error = %v", err)
	}
}
