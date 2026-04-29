package user

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ===========================================
// ==== Directory 实现 =====
// ===========================================

// directory 用户查询用例实现
type directory struct {
	uow uow.UnitOfWork
}

// NewDirectory 创建用户查询用例
func NewDirectory(uow uow.UnitOfWork) Directory {
	return &directory{uow: uow}
}

// GetByID 根据 ID 查询用户
func (s *directory) GetByID(ctx context.Context, userID string) (*UserResult, error) {
	l := logger.L(ctx)
	l.Debugw("查询用户信息",
		"action", logger.ActionRead,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	var result *UserResult

	// 查询操作也通过 UoW，但不需要写操作，可以直接使用只读事务
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionRead,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		user, err := tx.Users.FindByID(txCtx, userIDObj)
		if err != nil {
			l.Warnw("用户不存在",
				"action", logger.ActionRead,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			if isUserNotFound(err) {
				return perrors.WithCode(code.ErrUserNotFound, "user(%s) not found", userID)
			}
			return err
		}

		result = toUserResult(user)
		return nil
	})

	if err == nil {
		l.Debugw("查询用户成功",
			"action", logger.ActionRead,
			"resource", logger.ResourceUser,
			"user_id", result.ID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}

// GetByPhone 根据手机号查询用户
func (s *directory) GetByPhone(ctx context.Context, phone string) (*UserResult, error) {
	var result *UserResult

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		phoneObj, err := input.ParseOptionalPhone(phone)
		if err != nil {
			return err
		}

		user, err := tx.Users.FindByPhone(txCtx, phoneObj)
		if err != nil {
			return err
		}

		result = toUserResult(user)
		return nil
	})

	return result, err
}
