package user

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
)

// ===========================================
// ==== UserStatusApplicationService 实现 =====
// ===========================================

// userStatusApplicationService 用户状态应用服务实现
type userStatusApplicationService struct {
	uow            uow.UnitOfWork
	sessionManager session.Manager
}

// NewUserStatusApplicationService 创建用户状态应用服务
func NewUserStatusApplicationService(uow uow.UnitOfWork, sessionManager session.Manager) UserStatusApplicationService {
	return &userStatusApplicationService{uow: uow, sessionManager: sessionManager}
}

// Activate 激活用户
func (s *userStatusApplicationService) Activate(ctx context.Context, userID string) error {
	l := logger.L(ctx)
	l.Debugw("激活用户",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		lifecycler := user.NewLifecycler(tx.Users)

		// 转换 ID
		id, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务激活用户
		modifiedUser, err := lifecycler.Activate(ctx, id)
		if err != nil {
			l.Errorw("激活用户失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
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
func (s *userStatusApplicationService) Deactivate(ctx context.Context, userID string) error {
	l := logger.L(ctx)
	l.Debugw("停用用户",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		lifecycler := user.NewLifecycler(tx.Users)

		// 转换 ID
		id, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务停用用户
		modifiedUser, err := lifecycler.Deactivate(ctx, id)
		if err != nil {
			l.Errorw("停用用户失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
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
func (s *userStatusApplicationService) Block(ctx context.Context, userID string) error {
	l := logger.L(ctx)
	l.Debugw("封禁用户",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建领域服务
		lifecycler := user.NewLifecycler(tx.Users)

		// 转换 ID
		id, err := parseUserID(userID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务封禁用户
		modifiedUser, err := lifecycler.Block(ctx, id)
		if err != nil {
			l.Errorw("封禁用户失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(ctx, modifiedUser)
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
	id, parseErr := parseUserID(userID)
	if parseErr != nil {
		return parseErr
	}
	return s.sessionManager.RevokeByUser(ctx, id, "user_blocked", userID)
}
