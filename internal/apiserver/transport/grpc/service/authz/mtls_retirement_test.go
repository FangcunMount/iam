package authz

import (
	"context"
	"net"
	"testing"
	"time"

	authzv4 "github.com/FangcunMount/iam/v5/api/grpc/iam/authz/v4"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/authorization"
	servergrpc "github.com/FangcunMount/iam/v5/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v5/internal/testutil/tlsfixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestMTLSAuthorizationWithoutServiceToken(t *testing.T) {
	ca := tlsfixture.New(t)
	serverPair := ca.Issue(t, "server.test", false)
	cfg := servergrpc.NewConfig()
	cfg.Insecure = false
	cfg.TLSCertFile = serverPair.CertFile
	cfg.TLSKeyFile = serverPair.KeyFile
	cfg.MTLS.Enabled = true
	cfg.MTLS.CAFile = ca.CAFile
	cfg.MTLS.RequireClientCert = true
	cfg.MTLS.EnableAutoReload = false
	cfg.ACL.Enabled = true
	cfg.ACL.ConfigFile = "../../../../../../configs/grpc_acl.yaml"
	srv, err := servergrpc.NewServer(cfg)
	require.NoError(t, err)
	t.Cleanup(srv.Server.Stop)
	authzv4.RegisterAuthorizationServiceServer(srv.Server, &authorizationServer{checker: &checkerFake{decision: authorization.Decision{Allowed: true, Reason: authorization.ReasonAllowed}}})
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })
	go func() { _ = srv.Server.Serve(lis) }()
	valid := ca.Issue(t, "qs-apiserver.svc", false)
	unknown := ca.Issue(t, "unknown.svc", false)
	expired := ca.Issue(t, "qs-apiserver.svc", true)
	rogue := tlsfixture.New(t).Issue(t, "qs-apiserver.svc", false)
	for _, tt := range []struct {
		name   string
		pair   *tlsfixture.Pair
		bearer bool
		want   codes.Code
	}{
		{"certificate only", &valid, false, codes.OK}, {"bearer cannot change identity", &valid, true, codes.OK}, {"unknown identity", &unknown, true, codes.PermissionDenied}, {"missing certificate", nil, true, codes.Unavailable}, {"expired certificate", &expired, true, codes.Unavailable}, {"untrusted certificate", &rogue, true, codes.Unavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(ca.Client(tt.pair))))
			require.NoError(t, err)
			defer func() { _ = conn.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if tt.bearer {
				ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer forged-admin-token")
			}
			resp, err := authzv4.NewAuthorizationServiceClient(conn).Check(ctx, assessmentCheckRequest("user:2", "adhoc"))
			require.Equal(t, tt.want, status.Code(err))
			if tt.want == codes.OK {
				require.True(t, resp.Allowed)
			}
			if tt.name == "certificate only" {
				err = conn.Invoke(ctx, "/iam.authn.v3.AuthService/IssueServiceToken", &emptypb.Empty{}, &emptypb.Empty{})
				require.Equal(t, codes.Unimplemented, status.Code(err))
			}
		})
	}
}

func TestCommittedPolicyVersionRequiresQSServiceCertificate(t *testing.T) {
	ca := tlsfixture.New(t)
	serverPair := ca.Issue(t, "server.test", false)
	cfg := servergrpc.NewConfig()
	cfg.Insecure = false
	cfg.TLSCertFile = serverPair.CertFile
	cfg.TLSKeyFile = serverPair.KeyFile
	cfg.MTLS.Enabled = true
	cfg.MTLS.CAFile = ca.CAFile
	cfg.MTLS.RequireClientCert = true
	cfg.MTLS.EnableAutoReload = false
	cfg.ACL.Enabled = true
	cfg.ACL.ConfigFile = "../../../../../../configs/grpc_acl.yaml"
	srv, err := servergrpc.NewServer(cfg)
	require.NoError(t, err)
	t.Cleanup(srv.Server.Stop)
	authzv4.RegisterAuthorizationServiceServer(srv.Server, &authorizationServer{
		committedVersionReader: committedVersionReaderFunc(func(context.Context) (int64, error) { return 42, nil }),
	})
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })
	go func() { _ = srv.Server.Serve(lis) }()

	for _, test := range []struct {
		name string
		want codes.Code
	}{
		{name: "qs-apiserver.svc", want: codes.OK},
		{name: "qs-collection-server.svc", want: codes.PermissionDenied},
		{name: "unknown.svc", want: codes.PermissionDenied},
	} {
		t.Run(test.name, func(t *testing.T) {
			pair := ca.Issue(t, test.name, false)
			conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(ca.Client(&pair))))
			require.NoError(t, err)
			defer func() { _ = conn.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			response, err := authzv4.NewAuthorizationServiceClient(conn).GetCommittedPolicyVersion(ctx, &authzv4.GetCommittedPolicyVersionRequest{})
			require.Equal(t, test.want, status.Code(err))
			if test.want == codes.OK {
				require.EqualValues(t, 42, response.GetPolicyVersion())
			}
		})
	}
}
