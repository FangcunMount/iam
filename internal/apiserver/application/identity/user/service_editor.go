package user

import (
	"context"
	"strings"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/input"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
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

		modifiedUser, err := tx.Users.FindByID(txCtx, id)
		if err != nil {
			l.Errorw("修改用户名称失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		if err := modifiedUser.Rename(newName); err != nil {
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

		modifiedUser, err := tx.Users.FindByID(txCtx, id)
		if err != nil {
			l.Errorw("修改用户昵称失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		modifiedUser.UpdateNickname(newNickname)

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
		uniqueness := user.NewUniquenessChecker(tx.Users)

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

		modifiedUser, err := tx.Users.FindByID(txCtx, id)
		if err != nil {
			l.Errorw("更新联系方式失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		if phone.IsEmpty() {
			phone = modifiedUser.Phone
		}
		if email.IsEmpty() {
			email = modifiedUser.Email
		}
		if err := uniqueness.CheckPhoneChange(txCtx, modifiedUser, phone); err != nil {
			return err
		}
		modifiedUser.UpdatePhone(phone)
		modifiedUser.UpdateEmail(email)

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
		uniqueness := user.NewUniquenessChecker(tx.Users)

		id, err := parseUserID(dto.UserID)
		if err != nil {
			return err
		}

		var modifiedUser *user.User
		loadUser := func() (*user.User, error) {
			if modifiedUser != nil {
				return modifiedUser, nil
			}
			loaded, err := tx.Users.FindByID(txCtx, id)
			if err != nil {
				return nil, err
			}
			modifiedUser = loaded
			return modifiedUser, nil
		}

		if dto.Nickname != nil {
			nickname := strings.TrimSpace(*dto.Nickname)
			if nickname != "" {
				modifiedUser, err := loadUser()
				if err != nil {
					return err
				}
				modifiedUser.UpdateNickname(nickname)
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
			modifiedUser, err := loadUser()
			if err != nil {
				return err
			}
			if phone.IsEmpty() {
				phone = modifiedUser.Phone
			}
			if email.IsEmpty() {
				email = modifiedUser.Email
			}
			if err := uniqueness.CheckPhoneChange(txCtx, modifiedUser, phone); err != nil {
				return err
			}
			modifiedUser.UpdatePhone(phone)
			modifiedUser.UpdateEmail(email)
			if err := tx.Users.Update(txCtx, modifiedUser); err != nil {
				return err
			}
		}

		if modifiedUser == nil {
			modifiedUser, err = tx.Users.FindByID(txCtx, id)
			if err != nil {
				return err
			}
		}
		result = toUserResult(modifiedUser)
		return nil
	})
	return result, err
}
