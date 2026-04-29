package profile

import (
	"context"

	appProfileLink "github.com/FangcunMount/iam/internal/apiserver/application/uc/profilelink"
)

// ============= 应用服务接口（Driving Ports）=============

// ProfileApplicationService 档案命令应用服务。
type ProfileApplicationService interface {
	Register(ctx context.Context, dto RegisterProfileDTO) (*ProfileResult, error)
	Rename(ctx context.Context, profileID string, newName string) error
	UpdateIDCard(ctx context.Context, profileID string, name string, idCard string) error
	UpdateProfile(ctx context.Context, dto UpdateProfileDTO) error
	UpdateHeightWeight(ctx context.Context, dto UpdateHeightWeightDTO) error
}

// ProfileCreatorApplicationService 是创建档案的窄接口，供只需要建档能力的调用方使用。
type ProfileCreatorApplicationService interface {
	Register(ctx context.Context, dto RegisterProfileDTO) (*ProfileResult, error)
}

// ProfileRegistrationService 负责需要跨 Profile/ProfileLink 聚合的建档用例。
type ProfileRegistrationService interface {
	RegisterProfileWithProfileLink(ctx context.Context, dto RegisterProfileWithProfileLinkDTO) (*RegisterProfileWithProfileLinkResult, error)
}

// ProfileQueryApplicationService 档案查询应用服务。
type ProfileQueryApplicationService interface {
	GetByID(ctx context.Context, profileID string) (*ProfileResult, error)
	GetByIDCard(ctx context.Context, idCard string) (*ProfileResult, error)
	FindSimilar(ctx context.Context, name string, gender uint8, birthday string) ([]*ProfileResult, error)
}

// ProfileAccessApplicationService 当前用户视角的档案访问用例。
type ProfileAccessApplicationService interface {
	ListForProfileLink(ctx context.Context, userID string) ([]*ProfileResult, error)
	GetForProfileLink(ctx context.Context, userID string, profileID string) (*ProfileResult, error)
	PatchForProfileLink(ctx context.Context, dto PatchProfileForProfileLinkDTO) (*ProfileResult, error)
}

// ============= DTOs =============

// RegisterProfileDTO 创建档案 DTO。
type RegisterProfileDTO struct {
	Name     string
	Gender   uint8
	Birthday string
	IDCard   string
	Height   *uint32
	Weight   *uint32
}

// RegisterProfileWithProfileLinkDTO 同时创建档案并建立用户关系。
type RegisterProfileWithProfileLinkDTO struct {
	UserID   string
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

// PatchProfileForProfileLinkDTO 当前用户通过关系更新档案 DTO。
type PatchProfileForProfileLinkDTO struct {
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

// RegisterProfileWithProfileLinkResult 聚合 Profile 和 ProfileLink 的返回结果。
type RegisterProfileWithProfileLinkResult struct {
	Profile     *ProfileResult
	ProfileLink *appProfileLink.ProfileLinkResult
}
