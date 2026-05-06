package profile

import (
	"context"

	appProfileLink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
)

// ============= 当前调用者用例接口（Driving Ports）=============

// Creator 创建档案。
type Creator interface {
	Create(ctx context.Context, dto CreateProfileDTO) (*ProfileResult, error)
}

// Editor 编辑档案资料。
type Editor interface {
	Rename(ctx context.Context, profileID string, newName string) error
	UpdateIDCard(ctx context.Context, profileID string, name string, idCard string) error
	UpdateProfile(ctx context.Context, dto UpdateProfileDTO) error
	UpdateHeightWeight(ctx context.Context, dto UpdateHeightWeightDTO) error
}

// Directory 查询档案。
type Directory interface {
	GetByID(ctx context.Context, profileID string) (*ProfileResult, error)
	GetByIDCard(ctx context.Context, idCard string) (*ProfileResult, error)
	FindSimilar(ctx context.Context, name string, gender uint8, birthday string) ([]*ProfileResult, error)
}

// MyProfiles 当前用户视角的档案用例。
type MyProfiles interface {
	Create(ctx context.Context, currentUserID string, dto CreateMyProfileDTO) (*CreatedProfileResult, error)
	List(ctx context.Context, userID string) ([]*ProfileResult, error)
	Get(ctx context.Context, userID string, profileID string) (*ProfileResult, error)
	Patch(ctx context.Context, dto PatchMyProfileDTO) (*ProfileResult, error)
}

// ============= DTOs =============

// CreateProfileDTO 创建档案 DTO。
type CreateProfileDTO struct {
	Name     string
	Gender   uint8
	Birthday string
	IDCard   string
	Height   *uint32
	Weight   *uint32
}

// CreateMyProfileDTO 当前用户创建档案并建立关系 DTO。
type CreateMyProfileDTO struct {
	Name     string
	Gender   uint8
	Birthday string
	IDCard   string
	Height   *uint32
	Weight   *uint32
	Relation string
}

// UpdateProfileDTO 更新档案基础资料 DTO。
type UpdateProfileDTO struct {
	ProfileID string
	Gender    uint8
	Birthday  string
}

// UpdateHeightWeightDTO 更新档案身高体重 DTO。
type UpdateHeightWeightDTO struct {
	ProfileID string
	Height    uint32
	Weight    uint32
}

// PatchMyProfileDTO 当前用户通过关系更新档案 DTO。
type PatchMyProfileDTO struct {
	UserID    string
	ProfileID string
	LegalName *string
	Gender    *uint8
	Birthday  *string
	Height    *uint32
	Weight    *uint32
}

// ProfileResult 档案结果 DTO。
type ProfileResult struct {
	ID       string
	Name     string
	IDCard   string
	Gender   uint8
	Birthday string
	Height   uint32
	Weight   uint32
}

// CreatedProfileResult 聚合 Profile 和 ProfileLink 的返回结果。
type CreatedProfileResult struct {
	Profile     *ProfileResult
	ProfileLink *appProfileLink.ProfileLinkResult
}
