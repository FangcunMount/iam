package profile

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profile"
)

// ==============================================
// ==== ProfileApplicationService 实现 =====
// ==============================================

type profileApplicationService struct {
	uow uow.UnitOfWork
}

// NewProfileApplicationService 创建档案资料应用服务。
func NewProfileApplicationService(uow uow.UnitOfWork) ProfileApplicationService {
	return &profileApplicationService{uow: uow}
}

// Rename 修改档案姓名
func (s *profileApplicationService) Rename(ctx context.Context, profileID string, newName string) error {
	l := logger.L(ctx)

	l.Debugw("开始修改档案姓名",
		"action", logger.ActionUpdate,
		"resource", "profile",
		"resource_id", profileID,
		"new_name", newName,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := domain.NewValidator(tx.Profiles)
		profileService := domain.NewProfileService(tx.Profiles, validator)

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

		// 调用领域服务修改姓名
		modifiedProfile, err := profileService.Rename(ctx, id, newName)
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
func (s *profileApplicationService) UpdateIDCard(ctx context.Context, profileID string, name string, idCard string) error {
	l := logger.L(ctx)

	l.Debugw("开始更新档案身份证",
		"action", logger.ActionUpdate,
		"resource", "profile",
		"resource_id", profileID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := domain.NewValidator(tx.Profiles)
		profileService := domain.NewProfileService(tx.Profiles, validator)

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

		// 转换身份证
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

		// 调用领域服务更新身份证
		modifiedProfile, err := profileService.UpdateIDCard(ctx, id, idCardVO)
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
func (s *profileApplicationService) UpdateProfile(ctx context.Context, dto UpdateProfileDTO) error {
	l := logger.L(ctx)

	l.Debugw("开始更新档案基本信息",
		"action", logger.ActionUpdate,
		"resource", "profile",
		"resource_id", dto.ProfileID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := domain.NewValidator(tx.Profiles)
		profileService := domain.NewProfileService(tx.Profiles, validator)

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

		// 调用领域服务更新资料
		modifiedProfile, err := profileService.UpdateProfile(ctx, id, gender, birthday)
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

// UpdateHeightWeight 更新身高体重
func (s *profileApplicationService) UpdateHeightWeight(ctx context.Context, dto UpdateHeightWeightDTO) error {
	l := logger.L(ctx)

	l.Debugw("开始更新档案身高体重",
		"action", logger.ActionUpdate,
		"resource", "profile",
		"resource_id", dto.ProfileID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := domain.NewValidator(tx.Profiles)
		profileService := domain.NewProfileService(tx.Profiles, validator)

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
		height, err := input.ParseHeightCm(dto.Height)
		if err != nil {
			return err
		}
		// DTO中的Weight是克，需要转换为千克
		weight, err := input.ParseWeightGrams(dto.Weight)
		if err != nil {
			return err
		}

		// 调用领域服务更新身高体重
		modifiedProfile, err := profileService.UpdateHeightWeight(ctx, id, height, weight)
		if err != nil {
			l.Warnw("更新档案身高体重失败",
				"action", logger.ActionUpdate,
				"resource", "profile",
				"resource_id", dto.ProfileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Profiles.Update(txCtx, modifiedProfile)
	})

	if err == nil {
		l.Debugw("档案身高体重更新成功",
			"action", logger.ActionUpdate,
			"resource", "profile",
			"resource_id", dto.ProfileID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}
