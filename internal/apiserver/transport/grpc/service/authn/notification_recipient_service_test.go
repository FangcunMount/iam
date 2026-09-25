package authn

import (
	"context"
	"testing"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authnv3 "github.com/FangcunMount/iam/v5/api/grpc/iam/authn/v3"
	notificationrecipient "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/notificationrecipient"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recipientIdentityReaderStub struct{ calls int }

func (r *recipientIdentityReaderStub) ListActiveMiniProgramByUserIDs(_ context.Context, _ []meta.ID, _ string) ([]*loginidentity.LoginIdentity, error) {
	r.calls++
	return []*loginidentity.LoginIdentity{{ID: 21, UserID: 7, Provider: loginidentity.ProviderWechatMinip, Realm: "wx-allowed", Identifier: "private-openid", Status: loginidentity.StatusActive}}, nil
}

type recipientStatusReaderStub struct{}

func (recipientStatusReaderStub) ReadUserStatus(context.Context, meta.ID) (useraccess.Status, error) {
	return useraccess.StatusActive, nil
}

func TestMiniProgramRecipientLookupRequiresServiceAndAppScope(t *testing.T) {
	reader := &recipientIdentityReaderStub{}
	server := &notificationRecipientServiceServer{
		recipientQuery: notificationrecipient.NewQuery(reader, recipientStatusReaderStub{}),
		allowedAppIDs:  newNotificationAppIDSet([]string{"wx-allowed"}),
	}
	req := &authnv3.ResolveMiniProgramNotificationRecipientsRequest{UserIds: []string{"7"}, AppId: "wx-allowed"}
	for _, caller := range []string{"", "admin", "qs-collection-server.svc"} {
		ctx := context.Background()
		if caller != "" {
			ctx = interceptors.ContextWithServiceIdentity(ctx, &interceptors.ServiceIdentity{ServiceName: caller})
		}
		_, err := server.ResolveMiniProgramRecipients(ctx, req)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
	qs := interceptors.ContextWithServiceIdentity(context.Background(), &interceptors.ServiceIdentity{ServiceName: qsNotificationServiceName})
	_, err := server.ResolveMiniProgramRecipients(qs, &authnv3.ResolveMiniProgramNotificationRecipientsRequest{UserIds: []string{"7"}, AppId: "wx-other"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, reader.calls)

	resp, err := server.ResolveMiniProgramRecipients(qs, req)
	require.NoError(t, err)
	require.Equal(t, 1, reader.calls)
	require.Equal(t, []*authnv3.MiniProgramNotificationRecipient{{UserId: "7", LoginIdentityId: "21", AppId: "wx-allowed", OpenId: "private-openid"}}, resp.Items)
}

func TestMiniProgramRecipientLookupRejectsInvalidBatchesBeforeReadingIdentity(t *testing.T) {
	reader := &recipientIdentityReaderStub{}
	server := &notificationRecipientServiceServer{
		recipientQuery: notificationrecipient.NewQuery(reader, recipientStatusReaderStub{}),
		allowedAppIDs:  newNotificationAppIDSet([]string{"wx-allowed"}),
	}
	qs := interceptors.ContextWithServiceIdentity(context.Background(), &interceptors.ServiceIdentity{ServiceName: qsNotificationServiceName})
	for _, ids := range [][]string{{}, {"7", "7"}, {"0"}, {"not-an-id"}} {
		_, err := server.ResolveMiniProgramRecipients(qs, &authnv3.ResolveMiniProgramNotificationRecipientsRequest{UserIds: ids, AppId: "wx-allowed"})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	require.Zero(t, reader.calls)
}

func TestMiniProgramRecipientLookupDefaultsToDisabledWithoutAppAllowlist(t *testing.T) {
	reader := &recipientIdentityReaderStub{}
	server := &notificationRecipientServiceServer{
		recipientQuery: notificationrecipient.NewQuery(reader, recipientStatusReaderStub{}),
	}
	qs := interceptors.ContextWithServiceIdentity(context.Background(), &interceptors.ServiceIdentity{ServiceName: qsNotificationServiceName})
	_, err := server.ResolveMiniProgramRecipients(qs, &authnv3.ResolveMiniProgramNotificationRecipientsRequest{UserIds: []string{"7"}, AppId: "wx-allowed"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Zero(t, reader.calls)
}
