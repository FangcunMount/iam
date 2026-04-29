package container

import (
	"reflect"
	"testing"

	"github.com/FangcunMount/iam/internal/apiserver/container/assembler"
	authngrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/authn"
	authzgrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/authz"
	idpgrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/idp"
	ucgrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/uc"
	identitygrpc "github.com/FangcunMount/iam/internal/apiserver/transport/grpc/service/uc/identity"
	googlegrpc "google.golang.org/grpc"
)

func TestBuildGRPCDepsReturnsModuleRegistrarsInStartupOrder(t *testing.T) {
	c := &Container{
		AuthnModule: &assembler.AuthnModule{
			GRPCService: authngrpc.NewService(nil, nil, nil),
		},
		UserModule: &assembler.UserModule{
			GRPCService: ucgrpc.NewService(identitygrpc.NewService(nil, nil, nil, nil, nil, nil, nil, nil)),
		},
		IDPModule: &assembler.IDPModule{
			GRPCService: idpgrpc.NewService(nil, nil, nil),
		},
		AuthzModule: &assembler.AuthzModule{
			GRPCService: authzgrpc.NewService(nil, nil, nil, nil),
		},
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
