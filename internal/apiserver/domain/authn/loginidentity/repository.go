package loginidentity

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// Repository 登录身份仓储接口
type Repository interface {
	// Create 创建登录身份
	Create(ctx context.Context, identity *LoginIdentity) error

	// —— 查询登录身份 —— //
	// GetByID 根据ID获取登录身份
	GetByID(ctx context.Context, id meta.ID) (*LoginIdentity, error)
	// GetByProviderKey 根据提供者键获取登录身份
	GetByProviderKey(ctx context.Context, provider Provider, realm, identifier string) (*LoginIdentity, error)
	// GetByGlobalIdentifier 根据全局标识符获取登录身份
	GetByGlobalIdentifier(ctx context.Context, provider Provider, globalIdentifier string) (*LoginIdentity, error)
	// ListByUserID 根据用户ID获取登录身份列表
	ListByUserID(ctx context.Context, userID meta.ID) ([]*LoginIdentity, error)
}
