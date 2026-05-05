package user

import (
	"context"

	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
)

// ============= 当前调用者用例接口（Driving Ports）=============

// Creator 创建登录主体。
type Creator interface {
	// Create 创建新用户
	Create(ctx context.Context, dto CreateUserDTO) (*UserResult, error)
}

// Editor 编辑登录主体资料。
type Editor interface {
	// Rename 修改用户名称
	Rename(ctx context.Context, userID string, newName string) error
	// Renickname 修改用户昵称
	Renickname(ctx context.Context, userID string, newNickname string) error
	// PatchProfile 局部更新用户资料并返回最新用户结果
	PatchProfile(ctx context.Context, dto PatchUserProfileDTO) (*UserResult, error)
	// UpdateContact 更新联系方式
	UpdateContact(ctx context.Context, dto UpdateContactDTO) error
	// UpdateIDCard 更新身份证
	UpdateIDCard(ctx context.Context, userID string, idCard string) error
}

// StatusChanger 改变登录主体状态。
type StatusChanger interface {
	// Activate 激活用户
	Activate(ctx context.Context, userID string) error
	// Deactivate 停用用户
	Deactivate(ctx context.Context, userID string) error
	// Block 封禁用户
	Block(ctx context.Context, userID string) error
}

// Directory 查询登录主体。
type Directory interface {
	// GetByID 根据 ID 查询用户
	GetByID(ctx context.Context, userID string) (*UserResult, error)
	// BatchGetByID 根据 ID 集合批量查询用户，未找到或非法 ID 不会出现在返回 map 中。
	BatchGetByID(ctx context.Context, userIDs []string) (map[string]*UserResult, error)
	// GetByPhone 根据手机号查询用户
	GetByPhone(ctx context.Context, phone string) (*UserResult, error)
}

// ============= DTOs =============

// CreateUserDTO 创建用户 DTO
type CreateUserDTO struct {
	ID    uint64 // 用户ID（可选，0 表示由系统生成）
	Name  string // 用户名
	Phone string // 手机号（可选）
	Email string // 邮箱（可选）
}

// UpdateContactDTO 更新联系方式 DTO
type UpdateContactDTO struct {
	UserID string // 用户 ID
	Phone  string // 手机号（可选）
	Email  string // 邮箱（可选）
}

// PatchUserProfileDTO 局部更新用户资料 DTO。
type PatchUserProfileDTO struct {
	UserID   string
	Nickname *string
	Phone    *string
	Email    *string
}

// UserResult 用户结果 DTO
type UserResult struct {
	ID     string            // 用户 ID
	Name   string            // 用户名
	Phone  string            // 手机号
	Email  string            // 邮箱
	IDCard string            // 身份证号
	Status domain.UserStatus // 用户状态
}
