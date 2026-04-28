package credential

import (
	"context"
	"time"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ==================== Driven Ports (被驱动端口) ====================
// 由基础设施层实现，领域层使用

// Repository 凭据仓储接口（Driven Port）
// 职责：凭据持久化操作
type Repository interface {
	// Create 创建凭据
	Create(ctx context.Context, c *Credential) error

	// Update*** 更新凭据信息
	UpdateMaterial(ctx context.Context, id meta.ID, material []byte, algo string) error
	UpdateStatus(ctx context.Context, id meta.ID, status CredentialStatus) error
	UpdateFailedAttempts(ctx context.Context, id meta.ID, attempts int) error
	UpdateLockedUntil(ctx context.Context, id meta.ID, lockedUntil *time.Time) error
	UpdateLastSuccessAt(ctx context.Context, id meta.ID, lastSuccessAt time.Time) error
	UpdateLastFailureAt(ctx context.Context, id meta.ID, lastFailureAt time.Time) error
	UpdateExpiresAt(ctx context.Context, id meta.ID, expiresAt *time.Time) error

	// GetBy*** 查询凭据
	GetByID(ctx context.Context, id meta.ID) (*Credential, error)
	GetByAccountIDAndType(ctx context.Context, accountID meta.ID, credType CredentialType) (*Credential, error)
	GetByIDPIdentifier(ctx context.Context, idpIdentifier string, credType CredentialType) (*Credential, error)
	ListByAccountID(ctx context.Context, accountID meta.ID) ([]*Credential, error)

	// Delete 删除凭据
	Delete(ctx context.Context, id meta.ID) error
}
