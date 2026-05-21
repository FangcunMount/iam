package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// creator 用于创建会话。
type creator struct {
	store    Store
	lifetime LifetimePolicy
}

func newCreator(store Store, lifetime LifetimePolicy) *creator {
	return &creator{store: store, lifetime: lifetime}
}

// NewCreator 创建会话创建器。
func NewCreator(store Store, lifetime LifetimePolicy) Creator {
	return newCreator(store, lifetime)
}

func (c *creator) Create(ctx context.Context, principal *authentication.Principal) (*Session, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}
	expiresAt, err := c.lifetime.InitialExpiresAt(time.Now().UTC())
	if err != nil {
		return nil, err
	}

	session := New(uuid.NewString(), principal.UserID, principal.LoginIdentityID, principal.TenantID, principal.AMR, toStringClaims(principal.Claims), expiresAt)
	session.AuthMethod = principal.AuthMethod
	session.Realm = principal.Realm

	if err := c.store.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func toStringClaims(claims map[string]any) map[string]string {
	if len(claims) == 0 {
		return nil
	}
	out := make(map[string]string, len(claims))
	for key, value := range claims {
		if key == "" || value == nil {
			continue
		}
		out[key] = fmt.Sprint(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
