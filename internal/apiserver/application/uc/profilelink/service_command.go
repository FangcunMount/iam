package profilelink

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profilelink"
)

// =============================================
// ==== ProfileLinkApplicationService 实现 =====
// =============================================
// profileLinkApplicationService 档案关系应用服务实现
type profileLinkApplicationService struct {
	uow uow.UnitOfWork
}

// NewProfileLinkApplicationService 创建档案关系应用服务
func NewProfileLinkApplicationService(uow uow.UnitOfWork) ProfileLinkApplicationService {
	return &profileLinkApplicationService{uow: uow}
}

// CreateProfileLink 添加关系用户
func (s *profileLinkApplicationService) CreateProfileLink(ctx context.Context, dto CreateProfileLinkDTO) error {
	l := logger.L(ctx)
	l.Debugw("添加关系用户",
		"action", logger.ActionCreate,
		"resource", "profile_link",
		"user_id", dto.UserID,
		"profile_id", dto.ProfileID,
		"relation", dto.Relation,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		managerService := domain.NewManagerService(tx.ProfileLinks, tx.Profiles, tx.Users)

		// 转换 ID
		userID, err := parseUserID(dto.UserID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionCreate,
				"resource", "profile_link",
				"error", err.Error(),
			)
			return err
		}
		profileID, err := parseProfileID(dto.ProfileID)
		if err != nil {
			l.Warnw("档案ID格式错误",
				"action", logger.ActionCreate,
				"resource", "profile_link",
				"error", err.Error(),
			)
			return err
		}

		// 转换关系
		relation := domain.ParseRelation(dto.Relation)

		// 调用领域服务添加关系用户
		profileLink, err := managerService.CreateProfileLink(ctx, userID, profileID, relation)
		if err != nil {
			l.Errorw("添加关系用户失败",
				"action", logger.ActionCreate,
				"resource", "profile_link",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化档案关系
		return tx.ProfileLinks.Create(txCtx, profileLink)
	})

	if err == nil {
		l.Debugw("添加关系用户成功",
			"action", logger.ActionCreate,
			"resource", "profile_link",
			"user_id", dto.UserID,
			"profile_id", dto.ProfileID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}

// RemoveProfileLink 移除关系用户
func (s *profileLinkApplicationService) RemoveProfileLink(ctx context.Context, dto RemoveProfileLinkDTO) error {
	l := logger.L(ctx)
	l.Debugw("移除关系用户",
		"action", logger.ActionDelete,
		"resource", "profile_link",
		"user_id", dto.UserID,
		"profile_id", dto.ProfileID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 创建领域服务
		managerService := domain.NewManagerService(tx.ProfileLinks, tx.Profiles, tx.Users)

		// 转换 ID
		userID, err := parseUserID(dto.UserID)
		if err != nil {
			l.Warnw("用户ID格式错误",
				"action", logger.ActionDelete,
				"resource", "profile_link",
				"error", err.Error(),
			)
			return err
		}
		profileID, err := parseProfileID(dto.ProfileID)
		if err != nil {
			l.Warnw("档案ID格式错误",
				"action", logger.ActionDelete,
				"resource", "profile_link",
				"error", err.Error(),
			)
			return err
		}

		// 调用领域服务移除关系用户
		profileLink, err := managerService.RemoveProfileLink(ctx, userID, profileID)
		if err != nil {
			l.Errorw("移除关系用户失败",
				"action", logger.ActionDelete,
				"resource", "profile_link",
				"error", err.Error(),
				"result", logger.ResultFailed,
			)
			return err
		}

		// 持久化修改
		return tx.ProfileLinks.Update(txCtx, profileLink)
	})

	if err == nil {
		l.Debugw("移除关系用户成功",
			"action", logger.ActionDelete,
			"resource", "profile_link",
			"user_id", dto.UserID,
			"profile_id", dto.ProfileID,
			"result", logger.ResultSuccess,
		)
	}

	return err
}
