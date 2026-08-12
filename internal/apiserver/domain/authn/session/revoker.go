package session

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// revoker 用于撤销会话。
type revoker struct {
	store Store
}

// newRevoker 创建 revoker。
func newRevoker(store Store) *revoker {
	return &revoker{store: store}
}

// NewRevoker 创建会话撤销器。
func NewRevoker(store Store) Revoker {
	return newRevoker(store)
}

// Revoke 撤销指定会话。
func (r *revoker) Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error {
	return r.store.Revoke(ctx, sessionID, reason, revokedBy)
}

// RevokeByUser 撤销指定用户下的全部活跃会话。
func (r *revoker) RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error {
	return r.store.RevokeByUser(ctx, userID, reason, revokedBy)
}

// RevokeByLoginIdentity 撤销指定登录身份下的全部活跃会话。
func (r *revoker) RevokeByLoginIdentity(ctx context.Context, loginIdentityID meta.ID, reason string, revokedBy string) error {
	return r.store.RevokeByLoginIdentity(ctx, loginIdentityID, reason, revokedBy)
}
