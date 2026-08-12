package session

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ================== Session Role Interfaces ==================

// Creator 负责创建会话并应用初始生命周期规则。
type Creator interface {
	Create(ctx context.Context, principal *authentication.Principal) (*Session, error)
}

// Loader 负责读取会话，并统一判断会话是否仍然有效。
type Loader interface {
	Get(ctx context.Context, sessionID string) (*Session, error)
	GetActive(ctx context.Context, sessionID string) (*Session, error)
}

// Revoker 负责撤销单个或批量会话。
type Revoker interface {
	Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error
	RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error
	RevokeByLoginIdentity(ctx context.Context, loginIdentityID meta.ID, reason string, revokedBy string) error
}

// Extender 负责会话延期，并应用会话最大生命周期上限。
type Extender interface {
	Extend(ctx context.Context, sessionID string, expiresAt time.Time) error
	ExtendToRefreshExpiry(ctx context.Context, session *Session, refreshExpiresAt time.Time) error
}

// RefreshExpirer 负责计算下一次 refresh token 的领域过期时间。
type RefreshExpirer interface {
	NextRefreshExpiresAt(now time.Time, session *Session) (time.Time, error)
}

// SubjectAccessEvaluator 用于评估用户/登录身份的访问状态。
type SubjectAccessEvaluator interface {
	// —— 评估用户/登录身份的访问状态 —— //
	Evaluate(ctx context.Context, userID meta.ID, loginIdentityID meta.ID) (SubjectAccessDecision, error)
}
