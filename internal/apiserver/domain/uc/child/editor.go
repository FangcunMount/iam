package child

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ChildProfileEditor 儿童档案资料编辑应用服务
type ChildProfileEditor struct {
	repo      Repository
	validator Validator
}

// 确保 ChildProfileEditor 实现 ProfileEditor
var _ ProfileEditor = (*ChildProfileEditor)(nil)

// NewProfileService 创建儿童档案资料服务
func NewProfileService(repo Repository, validator Validator) *ChildProfileEditor {
	return &ChildProfileEditor{
		repo:      repo,
		validator: validator,
	}
}

// Rename 重命名儿童档案
// 领域逻辑：查询 + 修改实体
// 注意：不包括持久化，返回修改后的实体供应用层持久化
func (s *ChildProfileEditor) Rename(ctx context.Context, childID meta.ID, name string) (*Child, error) {
	// 验证名称
	if err := s.validator.ValidateRename(name); err != nil {
		return nil, err
	}

	child, err := s.repo.FindByID(ctx, childID)
	if err != nil {
		return nil, err
	}

	child.Rename(name)

	// 返回修改后的实体，由应用层持久化
	return child, nil
}

// UpdateIDCard 更新儿童身份证信息
// 领域逻辑：查询 + 修改实体
// 注意：不包括持久化，返回修改后的实体供应用层持久化
func (s *ChildProfileEditor) UpdateIDCard(ctx context.Context, childID meta.ID, idCard meta.IDCard) (*Child, error) {
	child, err := s.repo.FindByID(ctx, childID)
	if err != nil {
		return nil, err
	}

	// 领域逻辑：更新身份证
	child.UpdateIDCard(idCard)

	// 返回修改后的实体，由应用层持久化
	return child, nil
}

// UpdateProfile 更新儿童基础信息
// 领域逻辑：查询 + 修改实体
// 注意：不包括持久化，返回修改后的实体供应用层持久化
func (s *ChildProfileEditor) UpdateProfile(ctx context.Context, childID meta.ID, gender meta.Gender, birthday meta.Birthday) (*Child, error) {
	// 验证资料更新参数
	if err := s.validator.ValidateUpdateProfile(gender, birthday); err != nil {
		return nil, err
	}

	child, err := s.repo.FindByID(ctx, childID)
	if err != nil {
		return nil, err
	}

	// 领域逻辑：更新基础信息
	child.UpdateProfile(gender, birthday)

	// 返回修改后的实体，由应用层持久化
	return child, nil
}

// UpdateHeightWeight 更新儿童身高体重信息
// 领域逻辑：查询 + 修改实体
// 注意：不包括持久化，返回修改后的实体供应用层持久化
func (s *ChildProfileEditor) UpdateHeightWeight(ctx context.Context, childID meta.ID, height meta.Height, weight meta.Weight) (*Child, error) {
	child, err := s.repo.FindByID(ctx, childID)
	if err != nil {
		return nil, err
	}

	// 领域逻辑：更新身高体重
	child.UpdateHeightWeight(height, weight)

	// 返回修改后的实体，由应用层持久化
	return child, nil
}
