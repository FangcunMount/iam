package guardianship

import (
	"context"
)

// ============= 应用服务接口（Driving Ports）=============

// GuardianshipApplicationService 监护关系应用服务
type GuardianshipApplicationService interface {
	// AddGuardian 添加监护人
	AddGuardian(ctx context.Context, dto AddGuardianDTO) error
	// RemoveGuardian 移除监护人
	RemoveGuardian(ctx context.Context, dto RemoveGuardianDTO) error
}

// GuardianshipQueryApplicationService 监护关系查询应用服务（只读）
type GuardianshipQueryApplicationService interface {
	// IsGuardian 检查是否为监护人
	IsGuardian(ctx context.Context, userID string, childID string) (bool, error)
	// GetByUserIDAndChildID 查询监护关系
	GetByUserIDAndChildID(ctx context.Context, userID string, childID string) (*GuardianshipResult, error)
	// GetByUserIDAndChildIDIncludingRevoked 查询监护关系（包含已撤销）
	GetByUserIDAndChildIDIncludingRevoked(ctx context.Context, userID string, childID string) (*GuardianshipResult, error)
	// ListChildrenByUserID 列出用户监护的所有儿童
	ListChildrenByUserID(ctx context.Context, userID string) ([]*GuardianshipResult, error)
	// ListChildrenByUserIDIncludingRevoked 列出用户监护的所有儿童（包含已撤销）
	ListChildrenByUserIDIncludingRevoked(ctx context.Context, userID string) ([]*GuardianshipResult, error)
	// ListGuardiansByChildID 列出儿童的所有监护人
	ListGuardiansByChildID(ctx context.Context, childID string) ([]*GuardianshipResult, error)
	// ListGuardiansByChildIDIncludingRevoked 列出儿童的所有监护人（包含已撤销）
	ListGuardiansByChildIDIncludingRevoked(ctx context.Context, childID string) ([]*GuardianshipResult, error)
}

// GuardianshipAccessApplicationService 当前用户视角的监护关系访问用例。
type GuardianshipAccessApplicationService interface {
	GrantForCurrentUser(ctx context.Context, currentUserID string, dto AddGuardianDTO) (*GuardianshipResult, error)
	ListForCurrentUser(ctx context.Context, currentUserID string, dto ListGuardianshipsDTO) ([]*GuardianshipResult, error)
	RevokeBySelector(ctx context.Context, dto RevokeGuardianBySelectorDTO) (*GuardianshipResult, error)
}

// ============= DTOs =============

// AddGuardianDTO 添加监护人 DTO
type AddGuardianDTO struct {
	UserID   string // 用户 ID
	ChildID  string // 儿童 ID
	Relation string // 关系（self/parent/grandparent/other）
}

// RemoveGuardianDTO 移除监护人 DTO
type RemoveGuardianDTO struct {
	UserID  string // 用户 ID
	ChildID string // 儿童 ID
}

// ListGuardianshipsDTO 监护关系查询 DTO。
type ListGuardianshipsDTO struct {
	UserID  string
	ChildID string
	Active  *bool
}

// RevokeGuardianBySelectorDTO 通过 ID 或 user/child key 撤销监护关系。
type RevokeGuardianBySelectorDTO struct {
	GuardianshipID string
	UserID         string
	ChildID        string
}

// GuardianshipResult 监护关系结果 DTO
type GuardianshipResult struct {
	ID            uint64 // 监护关系 ID
	UserID        string // 用户 ID
	ChildID       string // 儿童 ID
	Relation      string // 关系
	EstablishedAt string // 建立时间
	RevokedAt     string // 撤销时间（为空表示未撤销）
	// 可选：包含儿童信息
	ChildName     string // 儿童姓名
	ChildGender   uint8  // 儿童性别（0=其他，1=男，2=女）
	ChildBirthday string // 儿童生日
}
