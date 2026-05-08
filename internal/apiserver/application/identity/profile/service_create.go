package profile

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
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
		// 构建创建信息
		info, err := buildProfileCreationInfo(dto)
		if err != nil {
			return err
		}

		// 校验建档信息
		if err := checkProfileCreationInfo(txCtx, tx.Profiles, info); err != nil {
			return err
		}

		// 创建档案记录
		newProfile, err := createProfileRecord(txCtx, tx.Profiles, info)
		if err != nil {
			return err
		}

		// 构建结果
		result = &ProfileResult{
			ID:       newProfile.ID.String(),
			Name:     newProfile.Name,
			Gender:   uint8(newProfile.Gender),
			Birthday: newProfile.Birthday.String(),
			IDCard:   newProfile.IDCard.Number(),
		}

		return nil
	})

	return result, err
}
