package profile

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
)

// ======================================
// ==== Editor 实现 =====
// ======================================

// NewCreator 创建档案创建用例。
func NewCreator(uow uow.UnitOfWork) Creator {
	return &profileEditor{uow: uow}
}

// Create 创建新档案
func (s *profileEditor) Create(ctx context.Context, dto CreateProfileDTO) (*ProfileResult, error) {
	l := logger.L(ctx)
	var result *ProfileResult

	l.Debugw("开始创建档案",
		"action", logger.ActionCreate,
		"resource", "profile",
		"profile_name", dto.Name,
		"has_idcard", dto.IDCard != "",
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		newProfile, err := buildProfileEntity(txCtx, tx, profileCreationInput{
			Name:     dto.Name,
			Gender:   dto.Gender,
			Birthday: dto.Birthday,
			IDCard:   dto.IDCard,
			Height:   dto.Height,
			Weight:   dto.Weight,
		})
		if err != nil {
			l.Errorw("创建档案实体失败",
				"action", logger.ActionCreate,
				"resource", "profile",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化档案
		if err := tx.Profiles.Create(txCtx, newProfile); err != nil {
			l.Errorw("持久化档案失败",
				"action", logger.ActionCreate,
				"resource", "profile",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 转换为 DTO
		result = toProfileResult(newProfile)
		return nil
	})

	if err == nil {
		l.Debugw("档案创建成功",
			"action", logger.ActionCreate,
			"resource", "profile",
			"resource_id", result.ID,
			"profile_name", result.Name,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}
