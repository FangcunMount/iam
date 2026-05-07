package profile

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/input"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	profiledomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
)

// ==============================================
// ==== Editor 实现 =====
// ==============================================

type profileEditor struct {
	uow uow.UnitOfWork
}

// NewEditor 创建档案资料用例。
func NewEditor(uow uow.UnitOfWork) Editor {
	return &profileEditor{uow: uow}
}

// Rename 修改档案姓名
func (s *profileEditor) Rename(ctx context.Context, profileID string, newName string) error {
	l := logger.L(ctx)

	l.Debugw("开始修改档案姓名",
		"action", logger.ActionUpdate,
		"resource", "profile",
		"resource_id", profileID,
		"new_name", newName,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 转换 ID
		id, err := parseProfileID(profileID)
		if err != nil {
			l.Warnw("档案ID解析失败",
				"action", logger.ActionUpdate,
				"resource", "profile",
				"resource_id", profileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		modifiedProfile, err := tx.Profiles.FindByID(txCtx, id)
		if err != nil {
			l.Warnw("修改档案姓名失败",
				"action", logger.ActionUpdate,
				"resource", "profile",
				"resource_id", profileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		if err := modifiedProfile.Rename(newName); err != nil {
			return err
		}

		// 持久化修改
		return tx.Profiles.Update(txCtx, modifiedProfile)
	})

	if err == nil {
		l.Debugw("档案姓名修改成功",
			"action", logger.ActionUpdate,
			"resource", "profile",
			"resource_id", profileID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// UpdateIDCard 更新身份证
func (s *profileEditor) UpdateIDCard(ctx context.Context, profileID string, name string, idCard string) error {
	l := logger.L(ctx)

	l.Debugw("开始更新档案身份证",
		"action", logger.ActionUpdate,
		"resource", "profile",
		"resource_id", profileID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 转换 ID
		id, err := parseProfileID(profileID)
		if err != nil {
			l.Warnw("档案ID解析失败",
				"action", logger.ActionUpdate,
				"resource", "profile",
				"resource_id", profileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		idCardVO, err := input.ParseIDCard(name, idCard)
		if err != nil {
			l.Warnw("身份证格式验证失败",
				"action", logger.ActionUpdate,
				"resource", "profile",
				"resource_id", profileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		modifiedProfile, err := tx.Profiles.FindByID(txCtx, id)
		if err != nil {
			l.Warnw("更新身份证失败",
				"action", logger.ActionUpdate,
				"resource", "profile",
				"resource_id", profileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		checker := profiledomain.NewIDCardUniquenessChecker(tx.Profiles)
		if err := checker.CheckIDCardChange(txCtx, modifiedProfile, idCardVO); err != nil {
			l.Warnw("身份证唯一性检查失败",
				"action", logger.ActionUpdate,
				"resource", "profile",
				"resource_id", profileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		modifiedProfile.UpdateIDCard(idCardVO)

		// 持久化修改
		return tx.Profiles.Update(txCtx, modifiedProfile)
	})

	if err == nil {
		l.Debugw("档案身份证更新成功",
			"action", logger.ActionUpdate,
			"resource", "profile",
			"resource_id", profileID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// UpdateProfile 更新基本信息（性别、生日）
func (s *profileEditor) UpdateProfile(ctx context.Context, dto UpdateProfileDTO) error {
	l := logger.L(ctx)

	l.Debugw("开始更新档案基本信息",
		"action", logger.ActionUpdate,
		"resource", "profile",
		"resource_id", dto.ProfileID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 转换 ID
		id, err := parseProfileID(dto.ProfileID)
		if err != nil {
			l.Warnw("档案ID解析失败",
				"action", logger.ActionUpdate,
				"resource", "profile",
				"resource_id", dto.ProfileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 转换值对象
		gender := input.ParseGender(dto.Gender)
		birthday := input.ParseBirthday(dto.Birthday)

		modifiedProfile, err := tx.Profiles.FindByID(txCtx, id)
		if err != nil {
			l.Warnw("更新档案基本信息失败",
				"action", logger.ActionUpdate,
				"resource", "profile",
				"resource_id", dto.ProfileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		modifiedProfile.UpdateProfile(gender, birthday)

		// 持久化修改
		return tx.Profiles.Update(txCtx, modifiedProfile)
	})

	if err == nil {
		l.Debugw("档案基本信息更新成功",
			"action", logger.ActionUpdate,
			"resource", "profile",
			"resource_id", dto.ProfileID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}
