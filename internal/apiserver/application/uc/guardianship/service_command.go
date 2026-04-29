package guardianship

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
)

// =============================================
// ==== GuardianshipApplicationService 实现 =====
// =============================================
// guardianshipApplicationService 监护关系应用服务实现
type guardianshipApplicationService struct {
	uow uow.UnitOfWork
}

// NewGuardianshipApplicationService 创建监护关系应用服务
func NewGuardianshipApplicationService(uow uow.UnitOfWork) GuardianshipApplicationService {
	return &guardianshipApplicationService{uow: uow}
}

// AddGuardian 添加监护人
func (s *guardianshipApplicationService) AddGuardian(ctx context.Context, dto AddGuardianDTO) error {
	l := logger.L(ctx)
	l.Debugw("添加监护人",
		"action", logger.ActionCreate,
		"resource", "guardianship",
		"user_id", dto.UserID,
		"child_id", dto.ChildID,
		"relation", dto.Relation,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		managerService := domain.NewManagerService(tx.Guardianships, tx.Children, tx.Users)

		// 转换 ID
		userID, err := parseUserID(dto.UserID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionCreate,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}
		childID, err := parseChildID(dto.ChildID)
		if err != nil {
			l.Warnw("儿童ID格式错误",
				"action", logger.ActionCreate,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}

		// 转换关系
		relation := domain.ParseRelation(dto.Relation)

		// 调用领域服务添加监护人
		guardianship, err := managerService.AddGuardian(ctx, userID, childID, relation)
		if err != nil {
			l.Errorw("添加监护人失败",
				"action", logger.ActionCreate,
				"resource", "guardianship",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化监护关系
		return tx.Guardianships.Create(ctx, guardianship)
	})

	if err == nil {
		l.Debugw("添加监护人成功",
			"action", logger.ActionCreate,
			"resource", "guardianship",
			"user_id", dto.UserID,
			"child_id", dto.ChildID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// RemoveGuardian 移除监护人
func (s *guardianshipApplicationService) RemoveGuardian(ctx context.Context, dto RemoveGuardianDTO) error {
	l := logger.L(ctx)
	l.Debugw("移除监护人",
		"action", logger.ActionDelete,
		"resource", "guardianship",
		"user_id", dto.UserID,
		"child_id", dto.ChildID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		managerService := domain.NewManagerService(tx.Guardianships, tx.Children, tx.Users)

		// 转换 ID
		userID, err := parseUserID(dto.UserID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionDelete,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}
		childID, err := parseChildID(dto.ChildID)
		if err != nil {
			l.Warnw("儿童ID格式错误",
				"action", logger.ActionDelete,
				"resource", "guardianship",
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务移除监护人
		guardianship, err := managerService.RemoveGuardian(ctx, userID, childID)
		if err != nil {
			l.Errorw("移除监护人失败",
				"action", logger.ActionDelete,
				"resource", "guardianship",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.Guardianships.Update(ctx, guardianship)
	})

	if err == nil {
		l.Debugw("移除监护人成功",
			"action", logger.ActionDelete,
			"resource", "guardianship",
			"user_id", dto.UserID,
			"child_id", dto.ChildID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}
