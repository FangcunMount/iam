package user

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ======================================
// ==== UserApplicationService 实现 =====
// ======================================

// userApplicationService 用户应用服务实现
type userApplicationService struct {
	uow uow.UnitOfWork
}

// NewUserApplicationService 创建用户应用服务
func NewUserApplicationService(uow uow.UnitOfWork) UserApplicationService {
	return &userApplicationService{uow: uow}
}

// Register 注册新用户
func (s *userApplicationService) Register(ctx context.Context, dto RegisterUserDTO) (*UserResult, error) {
	l := logger.L(ctx)
	var result *UserResult

	l.Debugw("开始注册用户",
		"action", logger.ActionRegister,
		"resource", logger.ResourceUser,
		"name", dto.Name,
		"phone", dto.Phone,
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建验证器
		validator := user.NewValidator(tx.Users)

		phone, err := input.ParseOptionalPhone(dto.Phone)
		if err != nil {
			return err
		}

		// 验证注册参数
		if err := validator.ValidateRegister(ctx, dto.Name, phone); err != nil {
			l.Warnw("用户注册参数验证失败",
				"action", logger.ActionRegister,
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
				"action", logger.ActionRegister,
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
					"action", logger.ActionRegister,
					"resource", logger.ResourceUser,
					"error", err.Error(),
				)
				return err
			}
			newUser.UpdateEmail(email)
		}

		// 持久化用户
		if err := tx.Users.Create(ctx, newUser); err != nil {
			l.Errorw("持久化用户失败",
				"action", logger.ActionRegister,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 转换为 DTO
		result = toUserResult(newUser)
		return nil
	})

	if err == nil {
		l.Debugw("用户注册成功",
			"action", logger.ActionRegister,
			"resource", logger.ResourceUser,
			"user_id", result.ID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}
