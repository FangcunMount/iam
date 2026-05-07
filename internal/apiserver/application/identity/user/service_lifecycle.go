package user

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ===========================================
// ==== StatusChanger 实现 =====
// ===========================================

// statusChanger 用户状态用例实现
type statusChanger struct {
	uow            uow.UnitOfWork
	sessionManager session.Manager
}

// NewStatusChanger 创建用户状态用例
func NewStatusChanger(uow uow.UnitOfWork, sessionManager session.Manager) StatusChanger {
	return &statusChanger{uow: uow, sessionManager: sessionManager}
}

// Activate 激活用户
func (s *statusChanger) Activate(ctx context.Context, userID meta.ID) error {
	l := logger.L(ctx)
	l.Debugw("激活用户",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		modifiedUser, err := tx.Users.FindByID(txCtx, userID)
		if err != nil {
			l.Errorw("激活用户失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		modifiedUser.Activate()

		// 持久化修改
		return tx.Users.Update(txCtx, modifiedUser)
	})

	if err == nil {
		l.Debugw("激活用户成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// Deactivate 停用用户
func (s *statusChanger) Deactivate(ctx context.Context, userID meta.ID) error {
	l := logger.L(ctx)
	l.Debugw("停用用户",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		modifiedUser, err := tx.Users.FindByID(txCtx, userID)
		if err != nil {
			l.Errorw("停用用户失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		modifiedUser.Deactivate()

		// 持久化修改
		return tx.Users.Update(txCtx, modifiedUser)
	})

	if err == nil {
		l.Debugw("停用用户成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// Block 封禁用户
func (s *statusChanger) Block(ctx context.Context, userID meta.ID) error {
	l := logger.L(ctx)
	l.Debugw("封禁用户",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		modifiedUser, err := tx.Users.FindByID(txCtx, userID)
		if err != nil {
			l.Errorw("封禁用户失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		modifiedUser.Block()

		// 持久化修改
		return tx.Users.Update(txCtx, modifiedUser)
	})

	if err == nil {
		l.Debugw("封禁用户成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	if err != nil {
		return err
	}
	if s.sessionManager == nil {
		return nil
	}
	return s.sessionManager.RevokeByUser(ctx, userID, "user_blocked", userID.String())
}
