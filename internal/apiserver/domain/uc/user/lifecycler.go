package user

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// lifecycler 用户生命周期管理服务实现
type lifecycler struct {
	repo Repository
}

// NewLifecycler 创建用户生命周期管理服务
func NewLifecycler(repo Repository) Lifecycler {
	return &lifecycler{repo: repo}
}

// Activate 激活用户
func (s *lifecycler) Activate(ctx context.Context, id meta.ID) (*User, error) {
	// 查找用户
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 激活用户
	user.Activate()

	return user, nil
}

// Deactivate 停用用户
func (s *lifecycler) Deactivate(ctx context.Context, id meta.ID) (*User, error) {
	// 查找用户
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 停用用户
	user.Deactivate()

	return user, nil
}

// Block 封禁用户
func (s *lifecycler) Block(ctx context.Context, id meta.ID) (*User, error) {
	// 查找用户
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 封禁用户
	user.Block()

	return user, nil
}
