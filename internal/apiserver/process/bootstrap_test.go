package process

import (
	"errors"
	"reflect"
	"testing"

	grpcpkg "github.com/FangcunMount/iam/v3/internal/pkg/grpc"
	genericapiserver "github.com/FangcunMount/iam/v3/internal/pkg/server"
)

func TestBootstrapTransportsBuildsAndRegistersInOrder(t *testing.T) {
	order := []string{}
	httpServer := &genericapiserver.GenericAPIServer{}
	grpcServer := &grpcpkg.Server{}

	output, err := bootstrapTransports(transportStageDeps{
		buildHTTPServer: func() (*genericapiserver.GenericAPIServer, error) {
			order = append(order, "build-http")
			return httpServer, nil
		},
		buildGRPCServer: func() (*grpcpkg.Server, error) {
			order = append(order, "build-grpc")
			return grpcServer, nil
		},
		registerREST: func(got *genericapiserver.GenericAPIServer) {
			if got != httpServer {
				t.Fatalf("REST server = %#v, want %#v", got, httpServer)
			}
			order = append(order, "register-rest")
		},
		registerGRPC: func(got *grpcpkg.Server) error {
			if got != grpcServer {
				t.Fatalf("gRPC server = %#v, want %#v", got, grpcServer)
			}
			order = append(order, "register-grpc")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("bootstrapTransports() error = %v", err)
	}
	if output.httpServer != httpServer || output.grpcServer != grpcServer {
		t.Fatalf("transport output = %#v, want configured servers", output)
	}
	want := []string{"build-http", "build-grpc", "register-rest", "register-grpc"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %#v, want %#v", order, want)
	}
}

func TestBootstrapTransportsReturnsGRPCRegistrationError(t *testing.T) {
	wantErr := errors.New("grpc register boom")

	_, err := bootstrapTransports(transportStageDeps{
		buildGRPCServer: func() (*grpcpkg.Server, error) {
			return &grpcpkg.Server{}, nil
		},
		registerGRPC: func(*grpcpkg.Server) error {
			return wantErr
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("bootstrapTransports() error = %v, want %v", err, wantErr)
	}
}

func TestRunPreparedServerReturnsFirstServiceError(t *testing.T) {
	wantErr := errors.New("http stopped")
	started := false

	err := runPreparedServer(preparedServerRunDeps{
		startShutdown: func() error {
			started = true
			return nil
		},
		transports: preparedServerTransports{
			runHTTP: func() error { return wantErr },
		},
	})
	if !started {
		t.Fatal("startShutdown was not called")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("runPreparedServer() error = %v, want %v", err, wantErr)
	}
}
