package user

import (
	"context"
	"strings"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/uow"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
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
func (s *editor) Rename(ctx context.Context, userID meta.ID, newName string) error {
	l := logger.L(ctx)
	l.Debugw("修改用户名称",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
		"new_name", newName,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		modifiedUser, err := tx.Users.FindByID(txCtx, userID)
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
func (s *editor) Renickname(ctx context.Context, userID meta.ID, newNickname string) error {
	l := logger.L(ctx)
	l.Debugw("修改用户昵称",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceUser,
		"user_id", userID,
		"new_nickname", newNickname,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		modifiedUser, err := tx.Users.FindByID(txCtx, userID)
		if err != nil {
			l.Errorw("修改用户昵称失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		modifiedUser.ChangeNickname(newNickname)

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
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		uniqueness := user.NewUniquenessChecker(tx.Users)

		phone, err := optionalPhone(dto.Phone)
		if err != nil {
			l.Warnw("手机号格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
			)
			return err
		}
		email, err := optionalEmail(dto.Email)
		if err != nil {
			l.Warnw("邮箱格式错误",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
			)
			return err
		}

		modifiedUser, err := tx.Users.FindByID(txCtx, dto.UserID)
		if err != nil {
			l.Errorw("更新联系方式失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceUser,
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
		modifiedUser.ChangePhone(phone)
		modifiedUser.ChangeEmail(email)

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

		var modifiedUser *user.User
		loadUser := func() (*user.User, error) {
			if modifiedUser != nil {
				return modifiedUser, nil
			}
			loaded, err := tx.Users.FindByID(txCtx, dto.UserID)
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
				modifiedUser.ChangeNickname(nickname)
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
			phone, err := optionalPhone(phoneValue)
			if err != nil {
				return err
			}
			email, err := optionalEmail(emailValue)
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
			modifiedUser.ChangePhone(phone)
			modifiedUser.ChangeEmail(email)
			if err := tx.Users.Update(txCtx, modifiedUser); err != nil {
				return err
			}
		}

		if modifiedUser == nil {
			var err error
			modifiedUser, err = tx.Users.FindByID(txCtx, dto.UserID)
			if err != nil {
				return err
			}
		}
		result = toUserResult(modifiedUser)
		return nil
	})
	return result, err
}
