package user

import (
	"context"
	"strings"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// =============================================
// ==== Editor 实现 =====
// =============================================

// editor 用户资料用例实现
type editor struct {
	uow uow.UnitOfWork
}

// NewEditor 创建用户资料用例
func NewEditor(uow uow.UnitOfWork) Editor {
	return &editor{uow: uow}
}

// Rename 修改用户名称
func (s *editor) Rename(ctx context.Context, userID string, newName string) error {
	l := logger.L(ctx)
	l.Debugw("修改用户名称",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
		"new_name", newName,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := user.NewValidator(tx.Users)
		profileEditor := user.NewProfileEditor(tx.Users, validator)

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

		// 调用领域服务修改名称
		modifiedUser, err := profileEditor.Rename(txCtx, id, newName)
		if err != nil {
			l.Errorw("修改用户名称失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(txCtx, modifiedUser)
	})

	if err == nil {
		l.Debugw("修改用户名称成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// Renickname 修改用户昵称
func (s *editor) Renickname(ctx context.Context, userID string, newNickname string) error {
	l := logger.L(ctx)
	l.Debugw("修改用户昵称",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
		"new_nickname", newNickname,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := user.NewValidator(tx.Users)
		profileEditor := user.NewProfileEditor(tx.Users, validator)

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

		// 调用领域服务修改昵称
		modifiedUser, err := profileEditor.Renickname(txCtx, id, newNickname)
		if err != nil {
			l.Errorw("修改用户昵称失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(txCtx, modifiedUser)
	})

	if err == nil {
		l.Debugw("修改用户昵称成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// UpdateContact 更新联系方式
func (s *editor) UpdateContact(ctx context.Context, dto UpdateContactDTO) error {
	l := logger.L(ctx)
	l.Debugw("更新用户联系方式",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", dto.UserID,
		"phone", dto.Phone,
		"email", dto.Email,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := user.NewValidator(tx.Users)
		profileEditor := user.NewProfileEditor(tx.Users, validator)

		// 转换 ID
		id, err := parseUserID(dto.UserID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		phone, err := input.ParseOptionalPhone(dto.Phone)
		if err != nil {
			l.Warnw("手机号格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}
		email, err := input.ParseOptionalEmail(dto.Email)
		if err != nil {
			l.Warnw("邮箱格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务更新联系方式
		modifiedUser, err := profileEditor.UpdateContact(txCtx, id, phone, email)
		if err != nil {
			l.Errorw("更新联系方式失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(txCtx, modifiedUser)
	})

	if err == nil {
		l.Debugw("更新联系方式成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", dto.UserID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// PatchProfile 局部更新用户资料并返回最新用户结果。
func (s *editor) PatchProfile(ctx context.Context, dto PatchUserProfileDTO) (*UserResult, error) {
	var result *UserResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		validator := user.NewValidator(tx.Users)
		profileEditor := user.NewProfileEditor(tx.Users, validator)

		id, err := parseUserID(dto.UserID)
		if err != nil {
			return err
		}

		if dto.Nickname != nil {
			nickname := strings.TrimSpace(*dto.Nickname)
			if nickname != "" {
				modifiedUser, err := profileEditor.Rename(txCtx, id, nickname)
				if err != nil {
					return err
				}
				if err := tx.Users.Update(txCtx, modifiedUser); err != nil {
					return err
				}
			}
		}

		var phoneValue, emailValue string
		var hasContact bool
		if dto.Phone != nil && *dto.Phone != "" {
			phoneValue = *dto.Phone
			hasContact = true
		}
		if dto.Email != nil && *dto.Email != "" {
			emailValue = *dto.Email
			hasContact = true
		}
		if hasContact {
			phone, err := input.ParseOptionalPhone(phoneValue)
			if err != nil {
				return err
			}
			email, err := input.ParseOptionalEmail(emailValue)
			if err != nil {
				return err
			}
			modifiedUser, err := profileEditor.UpdateContact(txCtx, id, phone, email)
			if err != nil {
				return err
			}
			if err := tx.Users.Update(txCtx, modifiedUser); err != nil {
				return err
			}
		}

		latest, err := tx.Users.FindByID(txCtx, id)
		if err != nil {
			return err
		}
		result = toUserResult(latest)
		return nil
	})
	return result, err
}

// UpdateIDCard 更新身份证
func (s *editor) UpdateIDCard(ctx context.Context, userID string, idCard string) error {
	l := logger.L(ctx)
	l.Debugw("更新用户身份证",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := user.NewValidator(tx.Users)
		profileEditor := user.NewProfileEditor(tx.Users, validator)

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

		// 转换身份证 (NewIDCard 需要name和number两个参数，这里我们只传number，name留空)
		idCardVO, err := meta.NewIDCard("", idCard)
		if err != nil {
			l.Warnw("身份证格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务更新身份证
		modifiedUser, err := profileEditor.UpdateIDCard(txCtx, id, idCardVO)
		if err != nil {
			l.Errorw("更新身份证失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Users.Update(txCtx, modifiedUser)
	})

	if err == nil {
		l.Debugw("更新身份证成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceUser,
			"user_id", userID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}
