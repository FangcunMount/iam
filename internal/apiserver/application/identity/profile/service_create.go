package profile

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	profiledomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
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
		idCard, hasIDCard, err := optionalIDCard(dto.Name, dto.IDCard)
		if err != nil {
			l.Errorw("档案身份证验证失败",
				"action", logger.ActionCreate,
				"resource", "profile",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}
		if hasIDCard {
			checker := profiledomain.NewIDCardUniquenessChecker(tx.Profiles)
			if err := checker.CheckIDCardUnique(txCtx, idCard); err != nil {
				l.Errorw("档案身份证唯一性检查失败",
					"action", logger.ActionCreate,
					"resource", "profile",
					"error", err.Error(),
					"result", logger.ResultFailed,
				)
				return err
			}
		}

		newProfile, err := buildProfileEntity(profileCreationInput{
			Name:     dto.Name,
			Gender:   meta.NewGender(dto.Gender),
			Birthday: meta.NewBirthday(dto.Birthday),
			IDCard:   idCard,
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
