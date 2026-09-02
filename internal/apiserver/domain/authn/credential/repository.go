package credential

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// AuthenticationState 是一次原子认证状态更新后的结果。
type AuthenticationState struct {
	FailedAttempts int
	LockedUntil    *time.Time
	NewlyLocked    bool
}

// MaterialRotation 描述认证成功时可选的凭据材料轮换。
type MaterialRotation struct {
	Material []byte
	Algo     *string
}

// ==================== Driven Ports (被驱动端口) ====================
// 由基础设施层实现，领域层使用

// Repository 凭据仓储接口（Driven Port）
// 职责：凭据持久化操作
type Repository interface {
	// Create 创建凭据
	Create(ctx context.Context, c *Credential) error

	// UpdateStatus 更新凭据状态
	UpdateStatus(ctx context.Context, id meta.ID, status CredentialStatus) error
	// ApplyAuthenticationTransition 原子执行认证状态迁移。
	ApplyAuthenticationTransition(ctx context.Context, transition AuthenticationTransition) (AuthenticationState, error)

	// GetByID 根据凭据ID查询凭据
	GetByID(ctx context.Context, id meta.ID) (*Credential, error)
	// GetByLoginIdentityIDAndType 根据登录身份ID和凭据类型查询密码类型凭据
	GetByLoginIdentityIDAndType(ctx context.Context, loginIdentityID meta.ID, credType CredentialType) (*Credential, error)
}
