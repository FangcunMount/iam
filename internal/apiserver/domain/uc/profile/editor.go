package profile

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// editor 封装档案资料编辑的领域规则。
type editor struct {
	repo      Repository
	validator Validator
}

var _ ProfileEditor = (*editor)(nil)

// NewProfileService 创建档案资料领域服务。
func NewProfileService(repo Repository, validator Validator) ProfileEditor {
	return &editor{
		repo:      repo,
		validator: validator,
	}
}

// Rename 重命名档案
// 领域逻辑：查询 + 修改实体
// 注意：不包括持久化，返回修改后的实体供应用层持久化
func (s *editor) Rename(ctx context.Context, profileID meta.ID, name string) (*Profile, error) {
	// 验证名称
	if err := s.validator.ValidateRename(name); err != nil {
		return nil, err
	}

	profile, err := s.repo.FindByID(ctx, profileID)
	if err != nil {
		return nil, err
	}

	profile.Rename(name)

	// 返回修改后的实体，由应用层持久化
	return profile, nil
}

// UpdateIDCard 更新档案身份证信息
// 领域逻辑：查询 + 修改实体
// 注意：不包括持久化，返回修改后的实体供应用层持久化
func (s *editor) UpdateIDCard(ctx context.Context, profileID meta.ID, idCard meta.IDCard) (*Profile, error) {
	profile, err := s.repo.FindByID(ctx, profileID)
	if err != nil {
		return nil, err
	}

	// 领域逻辑：更新身份证
	profile.UpdateIDCard(idCard)

	// 返回修改后的实体，由应用层持久化
	return profile, nil
}

// UpdateProfile 更新档案基础信息
// 领域逻辑：查询 + 修改实体
// 注意：不包括持久化，返回修改后的实体供应用层持久化
func (s *editor) UpdateProfile(ctx context.Context, profileID meta.ID, gender meta.Gender, birthday meta.Birthday) (*Profile, error) {
	// 验证资料更新参数
	if err := s.validator.ValidateUpdateProfile(gender, birthday); err != nil {
		return nil, err
	}

	profile, err := s.repo.FindByID(ctx, profileID)
	if err != nil {
		return nil, err
	}

	// 领域逻辑：更新基础信息
	profile.UpdateProfile(gender, birthday)

	// 返回修改后的实体，由应用层持久化
	return profile, nil
}

// UpdateHeightWeight 更新档案身高体重信息
// 领域逻辑：查询 + 修改实体
// 注意：不包括持久化，返回修改后的实体供应用层持久化
func (s *editor) UpdateHeightWeight(ctx context.Context, profileID meta.ID, height meta.Height, weight meta.Weight) (*Profile, error) {
	profile, err := s.repo.FindByID(ctx, profileID)
	if err != nil {
		return nil, err
	}

	// 领域逻辑：更新身高体重
	profile.UpdateHeightWeight(height, weight)

	// 返回修改后的实体，由应用层持久化
	return profile, nil
}
