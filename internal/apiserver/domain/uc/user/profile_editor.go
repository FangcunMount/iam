package user

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// profileEditor 用户资料编辑服务实现
type profileEditor struct {
	repo      Repository
	validator Validator
}

// NewProfileEditor 创建用户资料编辑服务
func NewProfileEditor(repo Repository, validator Validator) ProfileEditor {
	return &profileEditor{
		repo:      repo,
		validator: validator,
	}
}

// Rename 修改用户名称
func (s *profileEditor) Rename(ctx context.Context, id meta.ID, newName string) (*User, error) {
	// 验证名称
	if err := s.validator.ValidateRename(newName); err != nil {
		return nil, err
	}

	// 查找用户
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 修改名称
	user.Rename(newName)

	return user, nil
}

// Renickname 修改用户昵称
func (s *profileEditor) Renickname(ctx context.Context, id meta.ID, newNickname string) (*User, error) {
	// 查找用户
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 修改昵称
	user.UpdateNickname(newNickname)

	return user, nil
}

// UpdateContact 更新联系方式
func (s *profileEditor) UpdateContact(ctx context.Context, id meta.ID, phone meta.Phone, email meta.Email) (*User, error) {
	// 查找用户
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if phone.IsEmpty() {
		phone = user.Phone
	}
	if email.IsEmpty() {
		email = user.Email
	}

	// 验证联系方式变更
	if err := s.validator.ValidateUpdateContact(ctx, user, phone, email); err != nil {
		return nil, err
	}

	// 更新联系方式
	user.UpdatePhone(phone)
	user.UpdateEmail(email)

	return user, nil
}

// UpdateIDCard 更新身份证
func (s *profileEditor) UpdateIDCard(ctx context.Context, id meta.ID, idCard meta.IDCard) (*User, error) {
	// 查找用户
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新身份证
	user.UpdateIDCard(idCard)

	return user, nil
}
