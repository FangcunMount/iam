package credential

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ==================== Driven Ports (被驱动端口) ====================
// 由基础设施层实现，领域层使用

// Repository 凭据仓储接口（Driven Port）
// 职责：凭据持久化操作
type Repository interface {
	// Create 创建凭据
	Create(ctx context.Context, c *Credential) error

	UpdateMaterial(ctx context.Context, id meta.ID, material []byte, algo string) error
	UpdateStatus(ctx context.Context, id meta.ID, status CredentialStatus) error
	UpdateAuthState(ctx context.Context, c *Credential) error

	// GetBy*** 查询凭据
	GetByID(ctx context.Context, id meta.ID) (*Credential, error)
	GetByLoginIdentityIDAndType(ctx context.Context, loginIdentityID meta.ID, credType CredentialType) (*Credential, error)
}
