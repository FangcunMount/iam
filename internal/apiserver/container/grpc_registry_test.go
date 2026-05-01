package container

import (
	"reflect"
	"testing"

	"github.com/FangcunMount/iam/v2/internal/apiserver/container/assembler"
	googlegrpc "google.golang.org/grpc"
)

func TestBuildGRPCDepsReturnsModuleRegistrarsInStartupOrder(t *testing.T) {
	c := &Container{
		AuthnModule: &assembler.AuthnModule{},
		UserModule:  &assembler.UserModule{},
		IDPModule:   &assembler.IDPModule{},
		AuthzModule: &assembler.AuthzModule{},
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

	want := []string{"authn", "user", "idp", "authz"}
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
