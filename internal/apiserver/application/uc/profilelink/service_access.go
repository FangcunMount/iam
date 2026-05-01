package profilelink

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/uow"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
)

// ====================================================
// ==== MyProfileLinks 实现 =====
// ====================================================

type myProfileLinks struct {
	uow uow.UnitOfWork
}

// NewMyProfileLinks 创建当前用户视角的档案关系访问用例。
func NewMyProfileLinks(uow uow.UnitOfWork) MyProfileLinks {
	return &myProfileLinks{uow: uow}
}

func (s *myProfileLinks) Grant(ctx context.Context, currentUserID string, dto CreateProfileLinkDTO) (*ProfileLinkResult, error) {
	if dto.UserID != "" && dto.UserID != currentUserID {
		return nil, perrors.WithCode(code.ErrPermissionDenied, "cannot grant profile link for another user")
	}
	dto.UserID = currentUserID
	return NewCommands(s.uow).Establish(ctx, dto)
}

func (s *myProfileLinks) List(ctx context.Context, currentUserID string, dto ListProfileLinksDTO) ([]*ProfileLinkResult, error) {
	if dto.UserID != "" && dto.UserID != currentUserID {
		return nil, perrors.WithCode(code.ErrPermissionDenied, "cannot query profile links for another user")
	}
	dto.UserID = currentUserID
	query := NewDirectory(s.uow)

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

func (s *myProfileLinks) Revoke(ctx context.Context, currentUserID string, dto RevokeProfileLinkBySelectorDTO) (*ProfileLinkResult, error) {
	var result *ProfileLinkResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		currentUser, err := parseUserID(currentUserID)
		if err != nil {
			return err
		}
		if dto.ProfileLinkID == "" && dto.UserID == "" {
			dto.UserID = currentUserID
		}
		userID, profileID, err := resolveRevokeSelector(txCtx, tx, dto)
		if err != nil {
			return err
		}
		if userID != currentUser {
			return perrors.WithCode(code.ErrPermissionDenied, "cannot revoke profile link for another user")
		}
		revoked, err := revokeProfileLinkInTx(txCtx, tx, userID, profileID)
		result = revoked
		return err
	})
	return result, err
}

func getByUserIDAndProfileID(ctx context.Context, query Directory, dto ListProfileLinksDTO) (*ProfileLinkResult, error) {
	if dto.Active != nil && !*dto.Active {
		return query.GetIncludingRevoked(ctx, dto.UserID, dto.ProfileID)
	}
	return query.Get(ctx, dto.UserID, dto.ProfileID)
}

func listProfilesByUserID(ctx context.Context, query Directory, dto ListProfileLinksDTO) ([]*ProfileLinkResult, error) {
	if dto.Active != nil && !*dto.Active {
		return query.ListProfilesForUserIncludingRevoked(ctx, dto.UserID)
	}
	return query.ListProfilesForUser(ctx, dto.UserID)
}

func listProfileLinksByProfileID(ctx context.Context, query Directory, dto ListProfileLinksDTO) ([]*ProfileLinkResult, error) {
	if dto.Active != nil && !*dto.Active {
		return query.ListLinksForProfileIncludingRevoked(ctx, dto.ProfileID)
	}
	return query.ListLinksForProfile(ctx, dto.ProfileID)
}

func ensureActiveProfileLinkAccess(ctx context.Context, query Directory, userID string, profileID string) error {
	if _, err := query.Get(ctx, userID, profileID); err != nil {
		return perrors.WithCode(code.ErrPermissionDenied, "you are not an active profile link of this profile")
	}
	return nil
}
