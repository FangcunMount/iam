package profile

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profile"
)

// ======================================
// ==== Editor 实现 =====
// ======================================

// NewCreator 创建档案创建用例。
func NewCreator(uow uow.UnitOfWork) Creator {
	return &profileEditor{uow: uow}
}

// Register 创建新档案
func (s *profileEditor) Create(ctx context.Context, dto CreateProfileDTO) (*ProfileResult, error) {
	l := logger.L(ctx)
	var result *ProfileResult

	l.Debugw("开始创建档案",
		"action", logger.ActionRegister,
		"resource", "profile",
		"profile_name", dto.Name,
		"has_idcard", dto.IDCard != "",
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建验证器
		validator := domain.NewValidator(tx.Profiles)

		// 转换 DTO 为值对象
		gender := input.ParseGender(dto.Gender)
		birthday := input.ParseBirthday(dto.Birthday)

		// 验证创建参数
		if err := validator.ValidateRegister(ctx, dto.Name, gender, birthday); err != nil {
			l.Warnw("档案创建参数验证失败",
				"action", logger.ActionRegister,
				"resource", "profile",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 创建档案实体
		var newProfile *domain.Profile
		var err error
		var options []domain.ProfileOption
		options = append(options, domain.WithGender(gender), domain.WithBirthday(birthday))

		if idCard, ok, err := input.ParseOptionalIDCard(dto.Name, dto.IDCard); err != nil {
			l.Warnw("身份证格式验证失败",
				"action", logger.ActionRegister,
				"resource", "profile",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		} else if ok {
			options = append(options, domain.WithIDCard(idCard))
		}

		newProfile, err = domain.NewProfile(dto.Name, options...)
		if err != nil {
			l.Errorw("创建档案实体失败",
				"action", logger.ActionRegister,
				"resource", "profile",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 设置可选的身高体重
		if dto.Height != nil || dto.Weight != nil {
			height := newProfile.Height
			if dto.Height != nil {
				h, err := input.ParseHeightCm(*dto.Height)
				if err != nil {
					return err
				}
				height = h
			}
			weight := newProfile.Weight
			if dto.Weight != nil {
				// DTO中的Weight是克，需要转换为千克
				w, err := input.ParseWeightGrams(*dto.Weight)
				if err != nil {
					return err
				}
				weight = w
			}
			newProfile.UpdateHeightWeight(height, weight)
		}

		// 持久化档案
		if err := tx.Profiles.Create(txCtx, newProfile); err != nil {
			l.Errorw("持久化档案失败",
				"action", logger.ActionRegister,
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
			"action", logger.ActionRegister,
			"resource", "profile",
			"resource_id", result.ID,
			"profile_name", result.Name,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}
