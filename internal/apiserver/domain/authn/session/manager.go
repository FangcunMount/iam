package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// Manager 提供会话生命周期管理能力。
type Manager interface {
	Create(ctx context.Context, principal *authentication.Principal, expiresAt time.Time) (*Session, error)
	Get(ctx context.Context, sessionID string) (*Session, error)
	Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error
	RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error
	RevokeByAccount(ctx context.Context, accountID meta.ID, reason string, revokedBy string) error
	Extend(ctx context.Context, sessionID string, expiresAt time.Time) error
}

type manager struct {
	store Store
}

// NewManager 创建会话管理器。
func NewManager(store Store) Manager {
	return &manager{store: store}
}

func (m *manager) Create(ctx context.Context, principal *authentication.Principal, expiresAt time.Time) (*Session, error) {
	if principal == nil {
		return nil, fmt.Errorf("principal is nil")
	}
	session := New(uuid.NewString(), principal.UserID, principal.AccountID, principal.TenantID, principal.AMR, toStringClaims(principal.Claims), expiresAt)
	if err := m.store.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (m *manager) Get(ctx context.Context, sessionID string) (*Session, error) {
	return m.store.Get(ctx, sessionID)
}

func (m *manager) Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error {
	return m.store.Revoke(ctx, sessionID, reason, revokedBy)
}

func (m *manager) RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error {
	return m.store.RevokeByUser(ctx, userID, reason, revokedBy)
}

func (m *manager) RevokeByAccount(ctx context.Context, accountID meta.ID, reason string, revokedBy string) error {
	return m.store.RevokeByAccount(ctx, accountID, reason, revokedBy)
}

func (m *manager) Extend(ctx context.Context, sessionID string, expiresAt time.Time) error {
	return m.store.Extend(ctx, sessionID, expiresAt)
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
