package profilelink

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/identity/uow"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// ==================================================
// ==== Directory 实现 =====
// ==================================================

// directory 档案关系查询用例实现
type directory struct {
	uow uow.UnitOfWork
}

// NewDirectory 创建档案关系查询用例
func NewDirectory(uow uow.UnitOfWork) Directory {
	return &directory{uow: uow}
}

// IsLinked 检查是否为关系用户
func (s *directory) IsLinked(ctx context.Context, userID meta.ID, profileID meta.ID) (bool, error) {
	var hasProfileLink bool

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		var err error
		hasProfileLink, err = tx.ProfileLinks.IsLinked(txCtx, userID, profileID)
		return err
	})

	return hasProfileLink, err
}

// Get 查询档案关系
func (s *directory) Get(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLinkResult, error) {
	return s.getByUserIDAndProfileID(ctx, userID, profileID, false)
}

// GetIncludingRevoked 查询档案关系（包含已撤销）
func (s *directory) GetIncludingRevoked(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLinkResult, error) {
	return s.getByUserIDAndProfileID(ctx, userID, profileID, true)
}

func (s *directory) getByUserIDAndProfileID(ctx context.Context, userID meta.ID, profileID meta.ID, includeRevoked bool) (*ProfileLinkResult, error) {
	var result *ProfileLinkResult

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		var profileLink *domain.ProfileLink
		var err error
		if includeRevoked {
			profileLink, err = tx.ProfileLinks.FindByUserIDAndProfileIDIncludingRevoked(txCtx, userID, profileID)
		} else {
			profileLink, err = tx.ProfileLinks.FindByUserIDAndProfileID(txCtx, userID, profileID)
		}
		if err != nil {
			return err
		}

		// 查询档案信息
		profile, err := tx.Profiles.FindByID(txCtx, profileLink.Profile)
		if err != nil {
			return err
		}

		result = toProfileLinkResult(profileLink, profile)
		return nil
	})

	return result, err
}

// ListProfilesForUser 列出用户关系的所有档案
func (s *directory) ListProfilesForUser(ctx context.Context, userID meta.ID) ([]*ProfileLinkResult, error) {
	return s.listProfilesByUserID(ctx, userID, false)
}

// ListProfilesForUserIncludingRevoked 列出用户关系的所有档案（包含已撤销）
func (s *directory) ListProfilesForUserIncludingRevoked(ctx context.Context, userID meta.ID) ([]*ProfileLinkResult, error) {
	return s.listProfilesByUserID(ctx, userID, true)
}

func (s *directory) listProfilesByUserID(ctx context.Context, userID meta.ID, includeRevoked bool) ([]*ProfileLinkResult, error) {
	var results []*ProfileLinkResult

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		var profileLinks []*domain.ProfileLink
		var err error
		if includeRevoked {
			profileLinks, err = tx.ProfileLinks.FindByUserIDIncludingRevoked(txCtx, userID)
		} else {
			profileLinks, err = tx.ProfileLinks.FindByUserID(txCtx, userID)
		}
		if err != nil {
			return err
		}

		profilesByID, err := tx.Profiles.FindByIDs(txCtx, profileIDsFromLinks(profileLinks))
		if err != nil {
			return err
		}

		results = make([]*ProfileLinkResult, 0, len(profileLinks))
		for _, g := range profileLinks {
			if g == nil {
				continue
			}
			profile := profilesByID[g.Profile]
			if profile == nil {
				return perrors.WithCode(code.ErrIdentityProfileNotFound, "profile not found: %s", g.Profile.String())
			}
			results = append(results, toProfileLinkResult(g, profile))
		}

		return nil
	})

	return results, err
}

func profileIDsFromLinks(profileLinks []*domain.ProfileLink) []meta.ID {
	ids := make([]meta.ID, 0, len(profileLinks))
	seen := make(map[meta.ID]struct{}, len(profileLinks))
	for _, link := range profileLinks {
		if link == nil || link.Profile.IsZero() {
			continue
		}
		if _, ok := seen[link.Profile]; ok {
			continue
		}
		seen[link.Profile] = struct{}{}
		ids = append(ids, link.Profile)
	}
	return ids
}

// ListLinksForProfile 列出档案的所有关系用户
func (s *directory) ListLinksForProfile(ctx context.Context, profileID meta.ID) ([]*ProfileLinkResult, error) {
	return s.listProfileLinksByProfileID(ctx, profileID, false)
}

// ListLinksForProfileIncludingRevoked 列出档案的所有关系用户（包含已撤销）
func (s *directory) ListLinksForProfileIncludingRevoked(ctx context.Context, profileID meta.ID) ([]*ProfileLinkResult, error) {
	return s.listProfileLinksByProfileID(ctx, profileID, true)
}

func (s *directory) listProfileLinksByProfileID(ctx context.Context, profileID meta.ID, includeRevoked bool) ([]*ProfileLinkResult, error) {
	var results []*ProfileLinkResult

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		var profileLinks []*domain.ProfileLink
		var err error
		if includeRevoked {
			profileLinks, err = tx.ProfileLinks.FindByProfileIDIncludingRevoked(txCtx, profileID)
		} else {
			profileLinks, err = tx.ProfileLinks.FindByProfileID(txCtx, profileID)
		}
		if err != nil {
			return err
		}

		// 查询档案信息（所有档案关系共享同一个档案）
		profile, err := tx.Profiles.FindByID(txCtx, profileID)
		if err != nil {
			return err
		}

		results = make([]*ProfileLinkResult, 0, len(profileLinks))
		for _, g := range profileLinks {
			if g == nil {
				continue
			}
			results = append(results, toProfileLinkResult(g, profile))
		}

		return nil
	})

	return results, err
}
