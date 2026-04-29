package profilelink

import (
	"context"
)

// ============= 应用服务接口（Driving Ports）=============

// ProfileLinkApplicationService 档案关系应用服务
type ProfileLinkApplicationService interface {
	// CreateProfileLink 添加关系用户
	CreateProfileLink(ctx context.Context, dto CreateProfileLinkDTO) error
	// RemoveProfileLink 移除关系用户
	RemoveProfileLink(ctx context.Context, dto RemoveProfileLinkDTO) error
}

// ProfileLinkQueryApplicationService 档案关系查询应用服务（只读）
type ProfileLinkQueryApplicationService interface {
	// HasProfileLink 检查是否为关系用户
	HasProfileLink(ctx context.Context, userID string, profileID string) (bool, error)
	// GetByUserIDAndProfileID 查询档案关系
	GetByUserIDAndProfileID(ctx context.Context, userID string, profileID string) (*ProfileLinkResult, error)
	// GetByUserIDAndProfileIDIncludingRevoked 查询档案关系（包含已撤销）
	GetByUserIDAndProfileIDIncludingRevoked(ctx context.Context, userID string, profileID string) (*ProfileLinkResult, error)
	// ListProfilesByUserID 列出用户关系的所有档案
	ListProfilesByUserID(ctx context.Context, userID string) ([]*ProfileLinkResult, error)
	// ListProfilesByUserIDIncludingRevoked 列出用户关系的所有档案（包含已撤销）
	ListProfilesByUserIDIncludingRevoked(ctx context.Context, userID string) ([]*ProfileLinkResult, error)
	// ListProfileLinksByProfileID 列出档案的所有关系用户
	ListProfileLinksByProfileID(ctx context.Context, profileID string) ([]*ProfileLinkResult, error)
	// ListProfileLinksByProfileIDIncludingRevoked 列出档案的所有关系用户（包含已撤销）
	ListProfileLinksByProfileIDIncludingRevoked(ctx context.Context, profileID string) ([]*ProfileLinkResult, error)
}

// ProfileLinkAccessApplicationService 当前用户视角的档案关系访问用例。
type ProfileLinkAccessApplicationService interface {
	GrantForCurrentUser(ctx context.Context, currentUserID string, dto CreateProfileLinkDTO) (*ProfileLinkResult, error)
	ListForCurrentUser(ctx context.Context, currentUserID string, dto ListProfileLinksDTO) ([]*ProfileLinkResult, error)
	RevokeBySelector(ctx context.Context, dto RevokeProfileLinkBySelectorDTO) (*ProfileLinkResult, error)
}

// ============= DTOs =============

// CreateProfileLinkDTO 添加关系用户 DTO
type CreateProfileLinkDTO struct {
	UserID    string // 用户 ID
	ProfileID string // 档案 ID
	Relation  string // 关系（self/parent/grandparent/other）
}

// RemoveProfileLinkDTO 移除关系用户 DTO
type RemoveProfileLinkDTO struct {
	UserID    string // 用户 ID
	ProfileID string // 档案 ID
}

// ListProfileLinksDTO 档案关系查询 DTO。
type ListProfileLinksDTO struct {
	UserID    string
	ProfileID string
	Active    *bool
}

// RevokeProfileLinkBySelectorDTO 通过 ID 或 user/profile key 撤销档案关系。
type RevokeProfileLinkBySelectorDTO struct {
	ProfileLinkID string
	UserID        string
	ProfileID     string
}

// ProfileLinkResult 档案关系结果 DTO
type ProfileLinkResult struct {
	ID            uint64 // 档案关系 ID
	UserID        string // 用户 ID
	ProfileID     string // 档案 ID
	Relation      string // 关系
	EstablishedAt string // 建立时间
	RevokedAt     string // 撤销时间（为空表示未撤销）
	// 可选：包含档案信息
	ProfileName     string // 档案姓名
	ProfileGender   uint8  // 档案性别（0=其他，1=男，2=女）
	ProfileBirthday string // 档案生日
}
