package profilelink

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
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
		revoked, err := revokeProfileLinkInTx(txCtx, tx, dto.UserID, dto.ProfileID, nil)
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
		userID, profileID, existing, err := resolveRevokeSelector(txCtx, tx, dto)
		if err != nil {
			return err
		}
		revoked, err := revokeProfileLinkInTx(txCtx, tx, userID, profileID, existing)
		result = revoked
		return err
	})
	return result, err
}

func establishProfileLinkInTx(txCtx context.Context, tx uow.TxRepositories, dto CreateProfileLinkDTO) (*ProfileLinkResult, error) {
	if err := ensureProfileLinkParticipantsExist(txCtx, tx, dto.UserID, dto.ProfileID); err != nil {
		return nil, err
	}

	relation := domain.ParseRelation(dto.Relation)
	if relation == domain.RelSelf {
		if err := domain.NewSelfProfileGuard(tx.ProfileLinks).EnsureCanCreateSelf(txCtx, dto.UserID); err != nil {
			return nil, err
		}
	}

	linker := domain.NewLinker(tx.ProfileLinks)
	profileLink, err := linker.Link(txCtx, dto.UserID, dto.ProfileID, relation)
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

func revokeProfileLinkInTx(txCtx context.Context, tx uow.TxRepositories, userID, profileID meta.ID, existing *domain.ProfileLink) (*ProfileLinkResult, error) {
	linker := domain.NewLinker(tx.ProfileLinks)
	var profileLink *domain.ProfileLink
	var err error
	if existing != nil {
		profileLink, err = linker.RevokeLink(existing)
	} else {
		profileLink, err = linker.Revoke(txCtx, userID, profileID)
	}
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

func ensureProfileLinkParticipantsExist(txCtx context.Context, tx uow.TxRepositories, userID, profileID meta.ID) error {
	profile, err := tx.Profiles.FindByID(txCtx, profileID)
	if err != nil {
		return perrors.WrapC(err, code.ErrDatabase, "find profile failed")
	}
	if profile == nil {
		return perrors.WithCode(code.ErrUserInvalid, "profile not found")
	}

	user, err := tx.Users.FindByID(txCtx, userID)
	if err != nil {
		return perrors.WrapC(err, code.ErrDatabase, "find user failed")
	}
	if user == nil {
		return perrors.WithCode(code.ErrUserInvalid, "user not found")
	}

	return nil
}

func resolveRevokeSelector(txCtx context.Context, tx uow.TxRepositories, dto RevokeProfileLinkBySelectorDTO) (meta.ID, meta.ID, *domain.ProfileLink, error) {
	if !dto.ProfileLinkID.IsZero() {
		existing, err := tx.ProfileLinks.FindByID(txCtx, dto.ProfileLinkID)
		if err != nil {
			return 0, 0, nil, err
		}
		if existing == nil || !existing.IsActive() {
			return 0, 0, nil, perrors.WithCode(code.ErrIdentityProfileLinkNotFound, "active profile link not found")
		}
		return existing.User, existing.Profile, existing, nil
	}

	return dto.UserID, dto.ProfileID, nil, nil
}
