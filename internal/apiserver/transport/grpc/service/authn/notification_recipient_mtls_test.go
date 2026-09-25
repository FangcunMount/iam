package authn

import (
	"context"
	"net"
	"testing"
	"time"

	authnv3 "github.com/FangcunMount/iam/v5/api/grpc/iam/authn/v3"
	notificationrecipient "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/notificationrecipient"
	servergrpc "github.com/FangcunMount/iam/v5/internal/pkg/grpc"
	"github.com/FangcunMount/iam/v5/internal/testutil/tlsfixture"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestMiniProgramRecipientLookupRequiresMTLSACLAndAppAllowlist(t *testing.T) {
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
	reader := &recipientIdentityReaderStub{}
	authnv3.RegisterNotificationRecipientServiceServer(srv.Server, &notificationRecipientServiceServer{
		recipientQuery: notificationrecipient.NewQuery(reader, recipientStatusReaderStub{}),
		allowedAppIDs:  newNotificationAppIDSet([]string{"wx-allowed"}),
	})
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lis.Close() })
	go func() { _ = srv.Server.Serve(lis) }()

	for _, tt := range []struct {
		name, service, appID string
		bearer               bool
		want                 codes.Code
	}{
		{"qs allowed", "qs-apiserver.svc", "wx-allowed", false, codes.OK},
		{"qs wrong app", "qs-apiserver.svc", "wx-other", false, codes.PermissionDenied},
		{"collection denied", "qs-collection-server.svc", "wx-allowed", false, codes.PermissionDenied},
		{"unknown denied with forged bearer", "unknown.svc", "wx-allowed", true, codes.PermissionDenied},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pair := ca.Issue(t, tt.service, false)
			conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(ca.Client(&pair))))
			require.NoError(t, err)
			defer func() { _ = conn.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if tt.bearer {
				ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer forged-admin-token")
			}
			resp, err := authnv3.NewNotificationRecipientServiceClient(conn).ResolveMiniProgramRecipients(ctx,
				&authnv3.ResolveMiniProgramNotificationRecipientsRequest{UserIds: []string{"7"}, AppId: tt.appID})
			require.Equal(t, tt.want, status.Code(err))
			if tt.want == codes.OK {
				require.Len(t, resp.GetItems(), 1)
				require.Equal(t, "21", resp.GetItems()[0].GetLoginIdentityId())
			}
		})
	}
	require.Equal(t, 1, reader.calls)
}
