package notificationrecipient

import (
	"context"
	"fmt"
	"strings"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

const MaxUsers = 20

// IdentityReader is intentionally narrower than the login-identity repository.
// The application never lists another provider or another mini-program realm.
type IdentityReader interface {
	ListActiveMiniProgramByUserIDs(context.Context, []meta.ID, string) ([]*loginidentity.LoginIdentity, error)
}

type Recipient struct {
	UserID          meta.ID
	LoginIdentityID meta.ID
	AppID           string
	OpenID          string
}

type Query struct {
	identities IdentityReader
	users      useraccess.UserStatusReader
}

func NewQuery(identities IdentityReader, users useraccess.UserStatusReader) *Query {
	return &Query{identities: identities, users: users}
}

func (q *Query) Resolve(ctx context.Context, userIDs []meta.ID, appID string) ([]Recipient, error) {
	if q == nil || q.identities == nil || q.users == nil {
		return nil, fmt.Errorf("notification recipient query is unavailable")
	}
	appID = strings.TrimSpace(appID)
	if appID == "" || len(userIDs) == 0 || len(userIDs) > MaxUsers {
		return nil, fmt.Errorf("invalid notification recipient query")
	}
	seen := make(map[meta.ID]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID <= 0 {
			return nil, fmt.Errorf("invalid user ID")
		}
		if _, duplicate := seen[userID]; duplicate {
			return nil, fmt.Errorf("duplicate user ID")
		}
		seen[userID] = struct{}{}
	}

	identities, err := q.identities.ListActiveMiniProgramByUserIDs(ctx, userIDs, appID)
	if err != nil {
		return nil, fmt.Errorf("load notification identities: %w", err)
	}
	activeUsers := make(map[meta.ID]bool, len(userIDs))
	for _, userID := range userIDs {
		status, err := q.users.ReadUserStatus(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("load notification user status: %w", err)
		}
		activeUsers[userID] = status == useraccess.StatusActive
	}

	result := make([]Recipient, 0, len(identities))
	for _, identity := range identities {
		if identity == nil || !activeUsers[identity.UserID] || identity.ID.IsZero() ||
			identity.Provider != loginidentity.ProviderWechatMinip || identity.Realm != appID ||
			identity.Status != loginidentity.StatusActive || strings.TrimSpace(identity.Identifier) == "" ||
			identity.Meta[loginidentity.MetaLegacyIdentifierSemantics] == loginidentity.LegacyIdentifierOpenOrUnion {
			continue
		}
		result = append(result, Recipient{
			UserID: identity.UserID, LoginIdentityID: identity.ID,
			AppID: appID, OpenID: identity.Identifier,
		})
	}
	return result, nil
}
