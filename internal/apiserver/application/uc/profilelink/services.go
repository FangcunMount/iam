package profilelink

import (
	"context"
)

// ============= 当前调用者用例接口（Driving Ports）=============

// Commands 执行系统侧档案关系命令。
type Commands interface {
	// Establish 建立档案关系。
	Establish(ctx context.Context, dto CreateProfileLinkDTO) (*ProfileLinkResult, error)
	// Revoke 撤销档案关系。
	Revoke(ctx context.Context, dto RemoveProfileLinkDTO) (*ProfileLinkResult, error)
	// RevokeBySelector 通过关系 ID 或 user/profile key 撤销档案关系。
	RevokeBySelector(ctx context.Context, dto RevokeProfileLinkBySelectorDTO) (*ProfileLinkResult, error)
}

// Directory 查询档案关系。
type Directory interface {
	// IsLinked 检查是否为关系用户
	IsLinked(ctx context.Context, userID string, profileID string) (bool, error)
	// Get 查询档案关系
	Get(ctx context.Context, userID string, profileID string) (*ProfileLinkResult, error)
	// GetIncludingRevoked 查询档案关系（包含已撤销）
	GetIncludingRevoked(ctx context.Context, userID string, profileID string) (*ProfileLinkResult, error)
	// ListProfilesForUser 列出用户关系的所有档案
	ListProfilesForUser(ctx context.Context, userID string) ([]*ProfileLinkResult, error)
	// ListProfilesForUserIncludingRevoked 列出用户关系的所有档案（包含已撤销）
	ListProfilesForUserIncludingRevoked(ctx context.Context, userID string) ([]*ProfileLinkResult, error)
	// ListLinksForProfile 列出档案的所有关系用户
	ListLinksForProfile(ctx context.Context, profileID string) ([]*ProfileLinkResult, error)
	// ListLinksForProfileIncludingRevoked 列出档案的所有关系用户（包含已撤销）
	ListLinksForProfileIncludingRevoked(ctx context.Context, profileID string) ([]*ProfileLinkResult, error)
}

// MyProfileLinks 当前用户视角的档案关系访问用例。
type MyProfileLinks interface {
	Grant(ctx context.Context, currentUserID string, dto CreateProfileLinkDTO) (*ProfileLinkResult, error)
	List(ctx context.Context, currentUserID string, dto ListProfileLinksDTO) ([]*ProfileLinkResult, error)
	Revoke(ctx context.Context, currentUserID string, dto RevokeProfileLinkBySelectorDTO) (*ProfileLinkResult, error)
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
