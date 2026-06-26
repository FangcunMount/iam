package container

import (
	"reflect"
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authn"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/authz"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/identity"
	"github.com/FangcunMount/iam/v2/internal/apiserver/container/idp"
	googlegrpc "google.golang.org/grpc"
)

func TestBuildGRPCDepsReturnsModuleRegistrarsInStartupOrder(t *testing.T) {
	c := &Container{
		AuthnModule:    &authn.AuthnModule{},
		IdentityModule: &identity.IdentityModule{},
		IDPModule:      &idp.IDPModule{},
		AuthzModule:    &authz.AuthzModule{},
		initialized:    true,
	}

	deps := c.BuildGRPCDeps(nil)
	registrations := deps.Registrations
	got := make([]string, 0, len(registrations))
	server := googlegrpc.NewServer()
	defer server.Stop()

	for _, registration := range registrations {
		got = append(got, registration.Module)
		if registration.Description == "" {
			t.Fatalf("registration %q has empty description", registration.Module)
		}
		if registration.Register == nil {
			t.Fatalf("registration %q has nil register function", registration.Module)
		}
		registration.Register(server)
	}

	want := []string{"authn", "identity", "idp", "authz"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registrations = %#v, want %#v", got, want)
	}
}

func TestBuildGRPCDepsHandlesNilContainer(t *testing.T) {
	var c *Container
	if registrations := c.BuildGRPCDeps(nil).Registrations; len(registrations) != 0 {
		t.Fatalf("registrations = %#v, want empty", registrations)
	}
}
