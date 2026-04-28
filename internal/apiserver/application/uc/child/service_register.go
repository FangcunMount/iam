package child

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
)

// ======================================
// ==== ChildApplicationService 实现 =====
// ======================================

// childApplicationService 儿童应用服务实现
type childApplicationService struct {
	uow uow.UnitOfWork
}

// NewChildApplicationService 创建儿童应用服务
func NewChildApplicationService(uow uow.UnitOfWork) ChildApplicationService {
	return &childApplicationService{uow: uow}
}

// Register 注册新儿童档案
func (s *childApplicationService) Register(ctx context.Context, dto RegisterChildDTO) (*ChildResult, error) {
	l := logger.L(ctx)
	var result *ChildResult

	l.Debugw("开始注册儿童档案",
		"action", logger.ActionRegister,
		"resource", logger.ResourceChild,
		"child_name", dto.Name,
		"has_idcard", dto.IDCard != "",
	)

	err := s.uow.WithinTx(ctx, func(tx uow.TxRepositories) error {
		// 创建验证器
		validator := domain.NewValidator(tx.Children)

		// 转换 DTO 为值对象
		gender := input.ParseGender(dto.Gender)
		birthday := input.ParseBirthday(dto.Birthday)

		// 验证注册参数
		if err := validator.ValidateRegister(ctx, dto.Name, gender, birthday); err != nil {
			l.Warnw("儿童注册参数验证失败",
				"action", logger.ActionRegister,
				"resource", logger.ResourceChild,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 创建儿童实体
		var newChild *domain.Child
		var err error
		var options []domain.ChildOption
		options = append(options, domain.WithGender(gender), domain.WithBirthday(birthday))

		if idCard, ok, err := input.ParseOptionalIDCard(dto.Name, dto.IDCard); err != nil {
			l.Warnw("身份证格式验证失败",
				"action", logger.ActionRegister,
				"resource", logger.ResourceChild,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		} else if ok {
			options = append(options, domain.WithIDCard(idCard))
		}

		newChild, err = domain.NewChild(dto.Name, options...)
		if err != nil {
			l.Errorw("创建儿童实体失败",
				"action", logger.ActionRegister,
				"resource", logger.ResourceChild,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 设置可选的身高体重
		if dto.Height != nil || dto.Weight != nil {
			height := newChild.Height
			if dto.Height != nil {
				h, err := input.ParseHeightCm(*dto.Height)
				if err != nil {
					return err
				}
				height = h
			}
			weight := newChild.Weight
			if dto.Weight != nil {
				// DTO中的Weight是克，需要转换为千克
				w, err := input.ParseWeightGrams(*dto.Weight)
				if err != nil {
					return err
				}
				weight = w
			}
			newChild.UpdateHeightWeight(height, weight)
		}

		// 持久化儿童
		if err := tx.Children.Create(ctx, newChild); err != nil {
			l.Errorw("持久化儿童档案失败",
				"action", logger.ActionRegister,
				"resource", logger.ResourceChild,
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 转换为 DTO
		result = toChildResult(newChild)
		return nil
	})

	if err == nil {
		l.Debugw("儿童档案注册成功",
			"action", logger.ActionRegister,
			"resource", logger.ResourceChild,
			"resource_id", result.ID,
			"child_name", result.Name,
			"result", logger.ResultSuccess,
		)
	}

	return result, err
}
