package profilelink

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profilelink"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ====================================================
// ==== ProfileLinkAccessApplicationService 实现 =====
// ====================================================

type profileLinkAccessApplicationService struct {
	uow uow.UnitOfWork
}

// NewProfileLinkAccessApplicationService 创建当前用户视角的档案关系访问用例。
func NewProfileLinkAccessApplicationService(uow uow.UnitOfWork) ProfileLinkAccessApplicationService {
	return &profileLinkAccessApplicationService{uow: uow}
}

func (s *profileLinkAccessApplicationService) GrantForCurrentUser(ctx context.Context, currentUserID string, dto CreateProfileLinkDTO) (*ProfileLinkResult, error) {
	if dto.UserID != "" && dto.UserID != currentUserID {
		return nil, perrors.WithCode(code.ErrPermissionDenied, "cannot grant profile link for another user")
	}
	dto.UserID = currentUserID
	if err := NewProfileLinkApplicationService(s.uow).CreateProfileLink(ctx, dto); err != nil {
		return nil, err
	}
	return NewProfileLinkQueryApplicationService(s.uow).GetByUserIDAndProfileID(ctx, currentUserID, dto.ProfileID)
}

func (s *profileLinkAccessApplicationService) ListForCurrentUser(ctx context.Context, currentUserID string, dto ListProfileLinksDTO) ([]*ProfileLinkResult, error) {
	if dto.UserID != "" && dto.UserID != currentUserID {
		return nil, perrors.WithCode(code.ErrPermissionDenied, "cannot query profile links for another user")
	}
	dto.UserID = currentUserID
	query := NewProfileLinkQueryApplicationService(s.uow)

	switch {
	case dto.UserID != "" && dto.ProfileID != "":
		if err := ensureActiveProfileLinkAccess(ctx, query, currentUserID, dto.ProfileID); err != nil {
			return nil, err
		}
		result, err := getByUserIDAndProfileID(ctx, query, dto)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return []*ProfileLinkResult{}, nil
		}
		return []*ProfileLinkResult{result}, nil
	case dto.UserID != "":
		return listProfilesByUserID(ctx, query, dto)
	case dto.ProfileID != "":
		if err := ensureActiveProfileLinkAccess(ctx, query, currentUserID, dto.ProfileID); err != nil {
			return nil, err
		}
		return listProfileLinksByProfileID(ctx, query, dto)
	default:
		return listProfilesByUserID(ctx, query, dto)
	}
}

func (s *profileLinkAccessApplicationService) RevokeBySelector(ctx context.Context, dto RevokeProfileLinkBySelectorDTO) (*ProfileLinkResult, error) {
	var result *ProfileLinkResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		var userID meta.ID
		var profileID meta.ID
		if dto.ProfileLinkID != "" {
			refID, err := input.ParseUserID(dto.ProfileLinkID)
			if err != nil {
				return err
			}
			existing, err := tx.ProfileLinks.FindByID(txCtx, refID)
			if err != nil {
				return err
			}
			userID = existing.User
			profileID = existing.Profile
		} else {
			parsedUserID, err := parseUserID(dto.UserID)
			if err != nil {
				return err
			}
			parsedProfileID, err := parseProfileID(dto.ProfileID)
			if err != nil {
				return err
			}
			userID = parsedUserID
			profileID = parsedProfileID
		}

		managerService := domain.NewManagerService(tx.ProfileLinks, tx.Profiles, tx.Users)
		profileLink, err := managerService.RemoveProfileLink(ctx, userID, profileID)
		if err != nil {
			return err
		}
		if err := tx.ProfileLinks.Update(txCtx, profileLink); err != nil {
			return err
		}

		profile, err := tx.Profiles.FindByID(txCtx, profileLink.Profile)
		if err != nil {
			return err
		}
		result = toProfileLinkResult(profileLink, profile)
		return nil
	})
	return result, err
}

func getByUserIDAndProfileID(ctx context.Context, query ProfileLinkQueryApplicationService, dto ListProfileLinksDTO) (*ProfileLinkResult, error) {
	if dto.Active != nil && !*dto.Active {
		return query.GetByUserIDAndProfileIDIncludingRevoked(ctx, dto.UserID, dto.ProfileID)
	}
	return query.GetByUserIDAndProfileID(ctx, dto.UserID, dto.ProfileID)
}

func listProfilesByUserID(ctx context.Context, query ProfileLinkQueryApplicationService, dto ListProfileLinksDTO) ([]*ProfileLinkResult, error) {
	if dto.Active != nil && !*dto.Active {
		return query.ListProfilesByUserIDIncludingRevoked(ctx, dto.UserID)
	}
	return query.ListProfilesByUserID(ctx, dto.UserID)
}

func listProfileLinksByProfileID(ctx context.Context, query ProfileLinkQueryApplicationService, dto ListProfileLinksDTO) ([]*ProfileLinkResult, error) {
	if dto.Active != nil && !*dto.Active {
		return query.ListProfileLinksByProfileIDIncludingRevoked(ctx, dto.ProfileID)
	}
	return query.ListProfileLinksByProfileID(ctx, dto.ProfileID)
}

func ensureActiveProfileLinkAccess(ctx context.Context, query ProfileLinkQueryApplicationService, userID string, profileID string) error {
	if _, err := query.GetByUserIDAndProfileID(ctx, userID, profileID); err != nil {
		return perrors.WithCode(code.ErrPermissionDenied, "you are not an active profile link of this profile")
	}
	return nil
}
