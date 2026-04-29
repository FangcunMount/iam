package child

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
)

// ==============================================
// ==== ChildProfileApplicationService 实现 =====
// ==============================================

// childProfileApplicationService 儿童资料应用服务实现
type childProfileApplicationService struct {
	uow uow.UnitOfWork
}

// NewChildProfileApplicationService 创建儿童资料应用服务
func NewChildProfileApplicationService(uow uow.UnitOfWork) ChildProfileApplicationService {
	return &childProfileApplicationService{uow: uow}
}

// Rename 修改儿童姓名
func (s *childProfileApplicationService) Rename(ctx context.Context, childID string, newName string) error {
	l := logger.L(ctx)

	l.Debugw("开始修改儿童姓名",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceChild,
		"resource_id", childID,
		"new_name", newName,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := domain.NewValidator(tx.Children)
		profileService := domain.NewProfileService(tx.Children, validator)

		// 转换 ID
		id, err := parseChildID(childID)
		if err != nil {
			l.Warnw("儿童ID解析失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceChild,
				"resource_id", childID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 调用领域服务修改姓名
		modifiedChild, err := profileService.Rename(ctx, id, newName)
		if err != nil {
			l.Warnw("修改儿童姓名失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceChild,
				"resource_id", childID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Children.Update(ctx, modifiedChild)
	})

	if err == nil {
		l.Debugw("儿童姓名修改成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceChild,
			"resource_id", childID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// UpdateIDCard 更新身份证
func (s *childProfileApplicationService) UpdateIDCard(ctx context.Context, childID string, name string, idCard string) error {
	l := logger.L(ctx)

	l.Debugw("开始更新儿童身份证",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceChild,
		"resource_id", childID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := domain.NewValidator(tx.Children)
		profileService := domain.NewProfileService(tx.Children, validator)

		// 转换 ID
		id, err := parseChildID(childID)
		if err != nil {
			l.Warnw("儿童ID解析失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceChild,
				"resource_id", childID,
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
				"resource", logger.ResourceChild,
				"resource_id", childID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 调用领域服务更新身份证
		modifiedChild, err := profileService.UpdateIDCard(ctx, id, idCardVO)
		if err != nil {
			l.Warnw("更新身份证失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceChild,
				"resource_id", childID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Children.Update(ctx, modifiedChild)
	})

	if err == nil {
		l.Debugw("儿童身份证更新成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceChild,
			"resource_id", childID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// UpdateProfile 更新基本信息（性别、生日）
func (s *childProfileApplicationService) UpdateProfile(ctx context.Context, dto UpdateChildProfileDTO) error {
	l := logger.L(ctx)

	l.Debugw("开始更新儿童基本信息",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceChild,
		"resource_id", dto.ChildID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := domain.NewValidator(tx.Children)
		profileService := domain.NewProfileService(tx.Children, validator)

		// 转换 ID
		id, err := parseChildID(dto.ChildID)
		if err != nil {
			l.Warnw("儿童ID解析失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceChild,
				"resource_id", dto.ChildID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 转换值对象
		gender := input.ParseGender(dto.Gender)
		birthday := input.ParseBirthday(dto.Birthday)

		// 调用领域服务更新资料
		modifiedChild, err := profileService.UpdateProfile(ctx, id, gender, birthday)
		if err != nil {
			l.Warnw("更新儿童基本信息失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceChild,
				"resource_id", dto.ChildID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Children.Update(ctx, modifiedChild)
	})

	if err == nil {
		l.Debugw("儿童基本信息更新成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceChild,
			"resource_id", dto.ChildID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// UpdateHeightWeight 更新身高体重
func (s *childProfileApplicationService) UpdateHeightWeight(ctx context.Context, dto UpdateHeightWeightDTO) error {
	l := logger.L(ctx)

	l.Debugw("开始更新儿童身高体重",
		"action", logger.ActionUpdate,
		"resource", logger.ResourceChild,
		"resource_id", dto.ChildID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		validator := domain.NewValidator(tx.Children)
		profileService := domain.NewProfileService(tx.Children, validator)

		// 转换 ID
		id, err := parseChildID(dto.ChildID)
		if err != nil {
			l.Warnw("儿童ID解析失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceChild,
				"resource_id", dto.ChildID,
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
		modifiedChild, err := profileService.UpdateHeightWeight(ctx, id, height, weight)
		if err != nil {
			l.Warnw("更新儿童身高体重失败",
				"action", logger.ActionUpdate,
				"resource", logger.ResourceChild,
				"resource_id", dto.ChildID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Children.Update(ctx, modifiedChild)
	})

	if err == nil {
		l.Debugw("儿童身高体重更新成功",
			"action", logger.ActionUpdate,
			"resource", logger.ResourceChild,
			"resource_id", dto.ChildID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}
