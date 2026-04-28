package user

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ================== Domain Service Interfaces (Driving Ports) ==================
// 这些接口由领域层（领域服务）实现，供应用层调用

// Validator 用户验证器接口（Driving Port - 领域服务）
// 封装用户相关的验证规则和业务检查
type Validator interface {
	// ValidateRegister 验证注册参数
	ValidateRegister(ctx context.Context, name string, phone meta.Phone) error

	// ValidateRename 验证改名参数
	ValidateRename(name string) error

	// ValidateUpdateContact 验证更新联系方式参数
	ValidateUpdateContact(ctx context.Context, user *User, phone meta.Phone, email meta.Email) error

	// CheckPhoneUnique 检查手机号唯一性
	CheckPhoneUnique(ctx context.Context, phone meta.Phone) error
}

// ProfileEditor 用户资料编辑器接口（Driving Port - 领域服务）
// 负责用户资料的修改操作
type ProfileEditor interface {
	// Rename 修改用户名称
	Rename(ctx context.Context, id meta.ID, newName string) (*User, error)

	// Renickname 修改用户昵称
	Renickname(ctx context.Context, id meta.ID, newNickname string) (*User, error)

	// UpdateContact 更新联系方式
	UpdateContact(ctx context.Context, id meta.ID, phone meta.Phone, email meta.Email) (*User, error)

	// UpdateIDCard 更新身份证
	UpdateIDCard(ctx context.Context, id meta.ID, idCard meta.IDCard) (*User, error)
}

// Lifecycler 用户生命周期管理器接口（Driving Port - 领域服务）
// 负责用户状态的变更操作
type Lifecycler interface {
	// Activate 激活用户
	Activate(ctx context.Context, id meta.ID) (*User, error)

	// Deactivate 停用用户
	Deactivate(ctx context.Context, id meta.ID) (*User, error)

	// Block 封禁用户
	Block(ctx context.Context, id meta.ID) (*User, error)
}
