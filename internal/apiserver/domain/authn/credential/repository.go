package credential

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
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

	// —— 更新凭据 ——
	// UpdateStatus 更新凭据状态
	UpdateStatus(ctx context.Context, id meta.ID, status CredentialStatus) error
	// RecordAuthenticationFailure 原子递增失败次数并应用锁定策略。
	RecordAuthenticationFailure(ctx context.Context, id meta.ID, now time.Time, policy LockoutPolicy) (AuthenticationState, error)
	// RecordAuthenticationSuccess 原子清零失败次数并完成可选材料轮换。
	RecordAuthenticationSuccess(ctx context.Context, id meta.ID, now time.Time, rotation *MaterialRotation) error

	// —— 查询凭据 ——
	// GetByID 根据凭据ID查询凭据
	GetByID(ctx context.Context, id meta.ID) (*Credential, error)
	// GetByLoginIdentityIDAndType 根据登录身份ID和凭据类型查询密码类型凭据
	GetByLoginIdentityIDAndType(ctx context.Context, loginIdentityID meta.ID, credType CredentialType) (*Credential, error)
}
