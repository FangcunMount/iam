package profile

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
)

// ============================================
// ==== ProfileQueryApplicationService 实现 =====
// ============================================

// profileQueryApplicationService 档案查询应用服务实现
type profileQueryApplicationService struct {
	uow uow.UnitOfWork
}

// NewProfileQueryApplicationService 创建档案查询应用服务
func NewProfileQueryApplicationService(uow uow.UnitOfWork) ProfileQueryApplicationService {
	return &profileQueryApplicationService{uow: uow}
}

// GetByID 根据 ID 查询档案
func (s *profileQueryApplicationService) GetByID(ctx context.Context, profileID string) (*ProfileResult, error) {
	l := logger.L(ctx)
	var result *ProfileResult

	l.Debugw("开始查询档案",
		"action", logger.ActionRead,
		"resource", "profile",
		"resource_id", profileID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		profileIDObj, err := parseProfileID(profileID)
		if err != nil {
			l.Warnw("档案ID解析失败",
				"action", logger.ActionRead,
				"resource", "profile",
				"resource_id", profileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		profile, err := tx.Profiles.FindByID(txCtx, profileIDObj)
		if err != nil {
			l.Warnw("查询档案失败",
				"action", logger.ActionRead,
				"resource", "profile",
				"resource_id", profileID,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		result = toProfileResult(profile)
		return nil
	})

	if err == nil {
		l.Debugw("查询档案成功",
			"action", logger.ActionRead,
			"resource", "profile",
			"resource_id", profileID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}

// GetByIDCard 根据身份证查询档案
func (s *profileQueryApplicationService) GetByIDCard(ctx context.Context, idCard string) (*ProfileResult, error) {
	l := logger.L(ctx)
	var result *ProfileResult

	l.Debugw("开始根据身份证查询档案",
		"action", logger.ActionRead,
		"resource", "profile",
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		idCardObj, err := input.ParseIDCard("", idCard)
		if err != nil {
			l.Warnw("身份证格式验证失败",
				"action", logger.ActionRead,
				"resource", "profile",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		profile, err := tx.Profiles.FindByIDCard(txCtx, idCardObj)
		if err != nil {
			l.Warnw("根据身份证查询档案失败",
				"action", logger.ActionRead,
				"resource", "profile",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		result = toProfileResult(profile)
		return nil
	})

	if err == nil && result != nil {
		l.Debugw("根据身份证查询档案成功",
			"action", logger.ActionRead,
			"resource", "profile",
			"resource_id", result.ID,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}

// FindSimilar 查找相似档案（姓名、性别、生日）
func (s *profileQueryApplicationService) FindSimilar(ctx context.Context, name string, gender uint8, birthday string) ([]*ProfileResult, error) {
	l := logger.L(ctx)
	var results []*ProfileResult

	l.Debugw("开始查找相似档案",
		"action", logger.ActionList,
		"resource", "profile",
		"name", name,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		genderObj := input.ParseGender(gender)
		birthdayObj := input.ParseBirthday(birthday)

		profiles, err := tx.Profiles.FindSimilar(txCtx, name, genderObj, birthdayObj)
		if err != nil {
			l.Warnw("查找相似档案失败",
				"action", logger.ActionList,
				"resource", "profile",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		results = toProfileResults(profiles)
		return nil
	})

	if err == nil {
		l.Debugw("查找相似档案成功",
			"action", logger.ActionList,
			"resource", "profile",
			"count", len(results),
			"result", logger.ResultSuccess,
		)
	}

	return results, err
}
