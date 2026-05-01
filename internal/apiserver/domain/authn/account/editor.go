package account

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// editor 账号编辑器实现
// 职责：封装账号编辑的业务规则
// 不包含：事务管理（由应用层负责）
type editor struct {
	repo Repository
}

// 确保实现了 Editor 接口
var _ Editor = (*editor)(nil)

// NewEditor 创建账号编辑器实例
func NewEditor(repo Repository) Editor {
	return &editor{
		repo: repo,
	}
}

// SetUniqueID 设置唯一标识
func (e *editor) SetUniqueID(ctx context.Context, accountID meta.ID, uniqueID UnionID) (*Account, error) {
	// 检查唯一标识是否为空
	if uniqueID == "" {
		return nil, errors.WithCode(code.ErrInvalidUniqueID, "uniqueID cannot be empty")
	}
	// 查询账号
	account, err := e.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.WithCode(code.ErrNotFoundAccount, "account not found")
	}

	// 使用领域对象的方法设置 uniqueID
	if !account.SetUniqueID(uniqueID) {
		return nil, errors.WithCode(code.ErrUniqueIDExists, "uniqueID already set")
	}

	return account, nil
}

// UpdateProfile 更新账号资料
func (e *editor) UpdateProfile(ctx context.Context, accountID meta.ID, profile map[string]string) (*Account, error) {
	// 查询账号
	account, err := e.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.WithCode(code.ErrNotFoundAccount, "account not found")
	}

	// 检查是否可以更新
	if !account.CanUpdateProfile() {
		return nil, errors.WithCode(code.ErrInvalidArgument, "cannot update profile for deleted account")
	}

	// 使用领域对象的方法更新资料
	account.UpdateProfile(profile)

	return account, nil
}

// UpdateMeta 更新账号元数据
func (e *editor) UpdateMeta(ctx context.Context, accountID meta.ID, meta map[string]string) (*Account, error) {
	// 查询账号
	account, err := e.repo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, errors.WithCode(code.ErrNotFoundAccount, "account not found")
	}

	// 检查是否可以更新
	if !account.CanUpdateMeta() {
		return nil, errors.WithCode(code.ErrInvalidArgument, "cannot update meta for deleted account")
	}

	// 使用领域对象的方法更新元数据
	account.UpdateMeta(meta)

	return account, nil
}
