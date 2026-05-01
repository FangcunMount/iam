package profilelink

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// =============================================
// ==== Commands 实现 =====
// =============================================
// commands 档案关系用例实现
type commands struct {
	uow uow.UnitOfWork
}

// NewCommands 创建档案关系用例
func NewCommands(uow uow.UnitOfWork) Commands {
	return &commands{uow: uow}
}

// Establish 添加关系用户。
func (s *commands) Establish(ctx context.Context, dto CreateProfileLinkDTO) (*ProfileLinkResult, error) {
	l := logger.L(ctx)
	var result *ProfileLinkResult
	l.Debugw("添加关系用户",
		"action", logger.ActionCreate,
		"resource", "profile_link",
		"user_id", dto.UserID,
		"profile_id", dto.ProfileID,
		"relation", dto.Relation,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		created, err := establishProfileLinkInTx(txCtx, tx, dto)
		result = created
		return err
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

	if err != nil {
		l.Errorw("添加关系用户失败",
			"action", logger.ActionCreate,
			"resource", "profile_link",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
	}

	return result, err
}

// Revoke 移除关系用户。
func (s *commands) Revoke(ctx context.Context, dto RemoveProfileLinkDTO) (*ProfileLinkResult, error) {
	l := logger.L(ctx)
	var result *ProfileLinkResult
	l.Debugw("移除关系用户",
		"action", logger.ActionDelete,
		"resource", "profile_link",
		"user_id", dto.UserID,
		"profile_id", dto.ProfileID,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
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
		revoked, err := revokeProfileLinkInTx(txCtx, tx, userID, profileID)
		result = revoked
		return err
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

	if err != nil {
		l.Errorw("移除关系用户失败",
			"action", logger.ActionDelete,
			"resource", "profile_link",
			"error", err.Error(),
			"result", logger.ResultFailed,
		)
	}

	return result, err
}

func (s *commands) RevokeBySelector(ctx context.Context, dto RevokeProfileLinkBySelectorDTO) (*ProfileLinkResult, error) {
	var result *ProfileLinkResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		userID, profileID, err := resolveRevokeSelector(txCtx, tx, dto)
		if err != nil {
			return err
		}
		revoked, err := revokeProfileLinkInTx(txCtx, tx, userID, profileID)
		result = revoked
		return err
	})
	return result, err
}

func establishProfileLinkInTx(txCtx context.Context, tx uow.TxRepositories, dto CreateProfileLinkDTO) (*ProfileLinkResult, error) {
	userID, err := parseUserID(dto.UserID)
	if err != nil {
		return nil, err
	}
	profileID, err := parseProfileID(dto.ProfileID)
	if err != nil {
		return nil, err
	}
	linker := domain.NewLinker(tx.ProfileLinks, tx.Profiles, tx.Users)
	profileLink, err := linker.Establish(txCtx, userID, profileID, domain.ParseRelation(dto.Relation))
	if err != nil {
		return nil, err
	}
	if err := tx.ProfileLinks.Create(txCtx, profileLink); err != nil {
		return nil, err
	}
	profile, err := tx.Profiles.FindByID(txCtx, profileLink.Profile)
	if err != nil {
		return nil, err
	}
	return toProfileLinkResult(profileLink, profile), nil
}

func revokeProfileLinkInTx(txCtx context.Context, tx uow.TxRepositories, userID, profileID meta.ID) (*ProfileLinkResult, error) {
	linker := domain.NewLinker(tx.ProfileLinks, tx.Profiles, tx.Users)
	profileLink, err := linker.Revoke(txCtx, userID, profileID)
	if err != nil {
		return nil, err
	}
	if err := tx.ProfileLinks.Update(txCtx, profileLink); err != nil {
		return nil, err
	}
	profile, err := tx.Profiles.FindByID(txCtx, profileLink.Profile)
	if err != nil {
		return nil, err
	}
	return toProfileLinkResult(profileLink, profile), nil
}

func resolveRevokeSelector(txCtx context.Context, tx uow.TxRepositories, dto RevokeProfileLinkBySelectorDTO) (meta.ID, meta.ID, error) {
	if dto.ProfileLinkID != "" {
		profileLinkID, err := parseProfileLinkID(dto.ProfileLinkID)
		if err != nil {
			return 0, 0, err
		}
		existing, err := tx.ProfileLinks.FindByID(txCtx, profileLinkID)
		if err != nil {
			return 0, 0, err
		}
		return existing.User, existing.Profile, nil
	}

	userID, err := parseUserID(dto.UserID)
	if err != nil {
		return 0, 0, err
	}
	profileID, err := parseProfileID(dto.ProfileID)
	if err != nil {
		return 0, 0, err
	}
	return userID, profileID, nil
}
