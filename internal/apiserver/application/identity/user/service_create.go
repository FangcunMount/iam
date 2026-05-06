package user

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/input"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ======================================
// ==== Creator 实现 =====
// ======================================

// creator 用户用例实现
type creator struct {
	uow uow.UnitOfWork
}

// NewCreator 创建用户用例
func NewCreator(uow uow.UnitOfWork) Creator {
	return &creator{uow: uow}
}

// Create 创建新用户
func (s *creator) Create(ctx context.Context, dto CreateUserDTO) (*UserResult, error) {
	l := logger.L(ctx)
	var result *UserResult

	l.Debugw("开始创建用户",
		"action", logger.ActionCreate,
		"resource", logger.ResourceUser,
		"name", dto.Name,
		"phone", dto.Phone,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建验证器
		validator := user.NewValidator(tx.Users)

		phone, err := input.ParseOptionalPhone(dto.Phone)
		if err != nil {
			return err
		}

		// 验证创建参数
		if err := validator.ValidateCreate(txCtx, dto.Name, phone); err != nil {
			l.Warnw("用户创建参数验证失败",
				"action", logger.ActionCreate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 创建用户实体（如有指定ID则使用）
		var opts []user.UserOption
		if dto.ID > 0 {
			opts = append(opts, user.WithID(meta.FromUint64(dto.ID)))
		}
		newUser, err := user.NewUser(dto.Name, phone, opts...)
		if err != nil {
			l.Errorw("创建用户实体失败",
				"action", logger.ActionCreate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 设置可选的邮箱
		if dto.Email != "" {
			email, err := input.ParseOptionalEmail(dto.Email)
			if err != nil {
				l.Warnw("邮箱格式验证失败",
					"action", logger.ActionCreate,
					"resource", logger.ResourceUser,
					"error", err.Error(),
				)
				return err
			}
			newUser.UpdateEmail(email)
		}

		// 持久化用户
		if err := tx.Users.Create(txCtx, newUser); err != nil {
			l.Errorw("持久化用户失败",
				"action", logger.ActionCreate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		if err := profileLinkDomain.NewSelfProfileEnsurer(tx.Profiles, tx.ProfileLinks).Ensure(txCtx, newUser); err != nil {
			return err
		}

		// 转换为 DTO
		result = toUserResult(newUser)
		return nil
	})

	if err == nil {
		l.Debugw("用户创建成功",
			"action", logger.ActionCreate,
			"resource", logger.ResourceUser,
			"user_id", result.ID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}
