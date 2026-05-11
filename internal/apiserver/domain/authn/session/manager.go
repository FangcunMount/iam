package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Manager 提供会话生命周期管理能力。
type Manager interface {
	// 创建会话
	Create(ctx context.Context, principal *authentication.Principal, expiresAt time.Time) (*Session, error)
	// 获取会话
	Get(ctx context.Context, sessionID string) (*Session, error)
	// 撤销会话
	Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error
	// 撤销指定用户下的全部活跃会话
	RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error
	// 撤销指定登录身份下的全部活跃会话
	RevokeByLoginIdentity(ctx context.Context, loginIdentityID meta.ID, reason string, revokedBy string) error
	// 延长会话过期时间
	Extend(ctx context.Context, sessionID string, expiresAt time.Time) error
}

type manager struct {
	store Store
}

// NewManager 创建会话管理器。
func NewManager(store Store) Manager {
	return &manager{store: store}
}

// Create 创建会话。
func (m *manager) Create(ctx context.Context, principal *authentication.Principal, expiresAt time.Time) (*Session, error) {
	if principal == nil {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "principal is required")
	}
	if expiresAt.IsZero() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "expiresAt is required")
	}

	// 创建会话。
	session := New(uuid.NewString(), principal.UserID, principal.LoginIdentityID, principal.TenantID, principal.AMR, toStringClaims(principal.Claims), expiresAt)
	session.AuthMethod = principal.AuthMethod
	session.Realm = principal.Realm

	// 保存会话。
	if err := m.store.Save(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// Get 获取会话。
func (m *manager) Get(ctx context.Context, sessionID string) (*Session, error) {
	return m.store.Get(ctx, sessionID)
}

// Revoke 撤销会话。
func (m *manager) Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error {
	return m.store.Revoke(ctx, sessionID, reason, revokedBy)
}

// RevokeByUser 撤销指定用户下的全部活跃会话。
func (m *manager) RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error {
	return m.store.RevokeByUser(ctx, userID, reason, revokedBy)
}

// RevokeByLoginIdentity 撤销指定登录身份下的全部活跃会话。
func (m *manager) RevokeByLoginIdentity(ctx context.Context, loginIdentityID meta.ID, reason string, revokedBy string) error {
	return m.store.RevokeByLoginIdentity(ctx, loginIdentityID, reason, revokedBy)
}

// Extend 延长会话过期时间。
func (m *manager) Extend(ctx context.Context, sessionID string, expiresAt time.Time) error {
	return m.store.Extend(ctx, sessionID, expiresAt)
}

// toStringClaims 将任意映射转换为字符串映射。
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
