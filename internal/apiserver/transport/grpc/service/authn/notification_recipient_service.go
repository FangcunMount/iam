package authn

import (
	"context"
	"strings"

	"github.com/FangcunMount/component-base/pkg/grpc/interceptors"
	authnv3 "github.com/FangcunMount/iam/v5/api/grpc/iam/authn/v3"
	notificationrecipient "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/notificationrecipient"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const qsNotificationServiceName = "qs-apiserver.svc"

type notificationRecipientServiceServer struct {
	authnv3.UnimplementedNotificationRecipientServiceServer
	recipientQuery *notificationrecipient.Query
	allowedAppIDs  map[string]struct{}
}

func newNotificationAppIDSet(appIDs []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(appIDs))
	for _, appID := range appIDs {
		if appID = strings.TrimSpace(appID); appID != "" {
			allowed[appID] = struct{}{}
		}
	}
	return allowed
}

func (s *notificationRecipientServiceServer) ResolveMiniProgramRecipients(
	ctx context.Context,
	req *authnv3.ResolveMiniProgramNotificationRecipientsRequest,
) (*authnv3.ResolveMiniProgramNotificationRecipientsResponse, error) {
	identity, ok := interceptors.ServiceIdentityFromContext(ctx)
	if !ok || identity == nil || identity.ServiceName != qsNotificationServiceName {
		return nil, status.Error(codes.PermissionDenied, "notification recipient caller is not allowed")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	appID := strings.TrimSpace(req.GetAppId())
	if _, allowed := s.allowedAppIDs[appID]; !allowed || appID == "" {
		return nil, status.Error(codes.PermissionDenied, "notification app is not allowed")
	}
	if s.recipientQuery == nil {
		return nil, status.Error(codes.Unavailable, "notification recipient query is unavailable")
	}
	if len(req.GetUserIds()) == 0 || len(req.GetUserIds()) > notificationrecipient.MaxUsers {
		return nil, status.Error(codes.InvalidArgument, "invalid recipient count")
	}
	userIDs := make([]meta.ID, 0, len(req.GetUserIds()))
	seen := make(map[meta.ID]struct{}, len(req.GetUserIds()))
	for _, raw := range req.GetUserIds() {
		userID, err := parseRequiredMetaID(raw, "user_id")
		if err != nil || userID <= 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid user ID")
		}
		if _, duplicate := seen[userID]; duplicate {
			return nil, status.Error(codes.InvalidArgument, "duplicate user ID")
		}
		seen[userID] = struct{}{}
		userIDs = append(userIDs, userID)
	}
	recipients, err := s.recipientQuery.Resolve(ctx, userIDs, appID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "notification recipient query failed")
	}
	resp := &authnv3.ResolveMiniProgramNotificationRecipientsResponse{
		Items: make([]*authnv3.MiniProgramNotificationRecipient, 0, len(recipients)),
	}
	for _, recipient := range recipients {
		resp.Items = append(resp.Items, &authnv3.MiniProgramNotificationRecipient{
			UserId: recipient.UserID.String(), LoginIdentityId: recipient.LoginIdentityID.String(),
			AppId: recipient.AppID, OpenId: recipient.OpenID,
		})
	}
	return resp, nil
}
