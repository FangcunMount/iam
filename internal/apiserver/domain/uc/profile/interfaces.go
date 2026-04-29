package profile

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ================== Domain Service Interfaces (Driving Ports) ==================
// 这些接口由领域层（领域服务）实现，供应用层调用
// 按照功能职责拆分，遵循接口隔离原则

// Validator 档案验证器接口（Driving Port - 领域服务）
// 封装档案相关的验证规则和业务检查
type Validator interface {
	// ValidateRegister 验证注册参数
	ValidateRegister(ctx context.Context, name string, gender meta.Gender, birthday meta.Birthday) error

	// ValidateRename 验证改名参数
	ValidateRename(name string) error

	// ValidateUpdateProfile 验证资料更新参数
	ValidateUpdateProfile(gender meta.Gender, birthday meta.Birthday) error
}

// ProfileEditor 档案资料管理领域服务接口
// 负责档案编辑相关的领域逻辑
type ProfileEditor interface {
	Rename(ctx context.Context, profileID meta.ID, name string) (*Profile, error)
	UpdateIDCard(ctx context.Context, profileID meta.ID, idCard meta.IDCard) (*Profile, error)
	UpdateProfile(ctx context.Context, profileID meta.ID, gender meta.Gender, birthday meta.Birthday) (*Profile, error)
	UpdateHeightWeight(ctx context.Context, profileID meta.ID, height meta.Height, weight meta.Weight) (*Profile, error)
}
