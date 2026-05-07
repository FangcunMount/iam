package profile

import (
	"context"

	appProfileLink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ============= 当前调用者用例接口（Driving Ports）=============

// Creator 创建档案。
type Creator interface {
	Create(ctx context.Context, dto CreateProfileDTO) (*ProfileResult, error)
}

// Editor 编辑档案资料。
type Editor interface {
	// Rename 重命名档案
	Rename(ctx context.Context, profileID meta.ID, newName string) error
	// UpdateIDCard 更新档案身份证号码
	UpdateIDCard(ctx context.Context, profileID meta.ID, name string, idCard string) error
	// UpdateProfile 更新档案基础资料
	UpdateProfile(ctx context.Context, dto UpdateProfileDTO) error
}

// Directory 查询档案。
type Directory interface {
	// GetByID 根据 ID 查询档案
	GetByID(ctx context.Context, profileID meta.ID) (*ProfileResult, error)
	// GetByIDCard 根据身份证号码查询档案
	GetByIDCard(ctx context.Context, idCard string) (*ProfileResult, error)
	// FindSimilar 根据姓名、性别、生日查询可能的匹配档案列表，帮助用户找到可能的重复档案。
	FindSimilar(ctx context.Context, name string, gender uint8, birthday string) ([]*ProfileResult, error)
}

// MyProfiles 当前用户视角的档案用例。
type MyProfiles interface {
	// Create 创建档案并建立与当前用户的关系。relation 可选，表示关系类型，如 "self"、"family" 等。
	Create(ctx context.Context, currUserID meta.ID, dto CreateMyProfileDTO) (*CreatedProfileResult, error)
	// List 列出当前用户相关的所有档案及其关系。
	List(ctx context.Context, userID meta.ID) ([]*ProfileResult, error)
	// Get 获取当前用户与指定档案的关系和档案信息。
	Get(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileResult, error)
	// Patch 更新当前用户与指定档案的关系和/或档案信息。
	Patch(ctx context.Context, dto PatchMyProfileDTO) (*ProfileResult, error)
}

// ============= DTOs =============

// CreateProfileDTO 创建档案 DTO。
type CreateProfileDTO struct {
	Name     string
	Gender   uint8
	Birthday string
	IDCard   string
}

// CreateMyProfileDTO 当前用户创建档案并建立关系 DTO。
type CreateMyProfileDTO struct {
	Name     string
	Gender   uint8
	Birthday string
	IDCard   string
	Relation string
}

// UpdateProfileDTO 更新档案基础资料 DTO。
type UpdateProfileDTO struct {
	ProfileID meta.ID
	Gender    uint8
	Birthday  string
}

// PatchMyProfileDTO 当前用户通过关系更新档案 DTO。
type PatchMyProfileDTO struct {
	UserID    meta.ID
	ProfileID meta.ID
	LegalName *string
	Gender    *uint8
	Birthday  *string
}

// ProfileResult 档案结果 DTO。
type ProfileResult struct {
	ID       string
	Name     string
	IDCard   string
	Gender   uint8
	Birthday string
}

// CreatedProfileResult 聚合 Profile 和 ProfileLink 的返回结果。
type CreatedProfileResult struct {
	Profile     *ProfileResult
	ProfileLink *appProfileLink.ProfileLinkResult
}
