package session

import (
	"context"
	"time"

	"github.com/google/uuid"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
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

	session := NewWithContexts(
		uuid.NewString(), principal.UserID, principal.LoginIdentityID, principal.TenantID,
		principal.AuthContext, principal.TokenContext, expiresAt,
	)

	if err := c.store.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}
