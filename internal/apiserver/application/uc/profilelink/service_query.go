package profilelink

import (
	"context"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profilelink"
)

// ==================================================
// ==== ProfileLinkQueryApplicationService 实现 =====
// ==================================================

// profileLinkQueryApplicationService 档案关系查询应用服务实现
type profileLinkQueryApplicationService struct {
	uow uow.UnitOfWork
}

// NewProfileLinkQueryApplicationService 创建档案关系查询应用服务
func NewProfileLinkQueryApplicationService(uow uow.UnitOfWork) ProfileLinkQueryApplicationService {
	return &profileLinkQueryApplicationService{uow: uow}
}

// HasProfileLink 检查是否为关系用户
func (s *profileLinkQueryApplicationService) HasProfileLink(ctx context.Context, userID string, profileID string) (bool, error) {
	var hasProfileLink bool

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			return err
		}

		profileIDObj, err := parseProfileID(profileID)
		if err != nil {
			return err
		}

		hasProfileLink, err = tx.ProfileLinks.HasProfileLink(txCtx, userIDObj, profileIDObj)
		return err
	})

	return hasProfileLink, err
}

// GetByUserIDAndProfileID 查询档案关系
func (s *profileLinkQueryApplicationService) GetByUserIDAndProfileID(ctx context.Context, userID string, profileID string) (*ProfileLinkResult, error) {
	return s.getByUserIDAndProfileID(ctx, userID, profileID, false)
}

// GetByUserIDAndProfileIDIncludingRevoked 查询档案关系（包含已撤销）
func (s *profileLinkQueryApplicationService) GetByUserIDAndProfileIDIncludingRevoked(ctx context.Context, userID string, profileID string) (*ProfileLinkResult, error) {
	return s.getByUserIDAndProfileID(ctx, userID, profileID, true)
}

func (s *profileLinkQueryApplicationService) getByUserIDAndProfileID(ctx context.Context, userID string, profileID string, includeRevoked bool) (*ProfileLinkResult, error) {
	var result *ProfileLinkResult

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			return err
		}

		profileIDObj, err := parseProfileID(profileID)
		if err != nil {
			return err
		}

		var profileLink *domain.ProfileLink
		if includeRevoked {
			profileLink, err = tx.ProfileLinks.FindByUserIDAndProfileIDIncludingRevoked(txCtx, userIDObj, profileIDObj)
		} else {
			profileLink, err = tx.ProfileLinks.FindByUserIDAndProfileID(txCtx, userIDObj, profileIDObj)
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

// ListProfilesByUserID 列出用户关系的所有档案
func (s *profileLinkQueryApplicationService) ListProfilesByUserID(ctx context.Context, userID string) ([]*ProfileLinkResult, error) {
	return s.listProfilesByUserID(ctx, userID, false)
}

// ListProfilesByUserIDIncludingRevoked 列出用户关系的所有档案（包含已撤销）
func (s *profileLinkQueryApplicationService) ListProfilesByUserIDIncludingRevoked(ctx context.Context, userID string) ([]*ProfileLinkResult, error) {
	return s.listProfilesByUserID(ctx, userID, true)
}

func (s *profileLinkQueryApplicationService) listProfilesByUserID(ctx context.Context, userID string, includeRevoked bool) ([]*ProfileLinkResult, error) {
	var results []*ProfileLinkResult

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		userIDObj, err := parseUserID(userID)
		if err != nil {
			return err
		}

		var profileLinks []*domain.ProfileLink
		if includeRevoked {
			profileLinks, err = tx.ProfileLinks.FindByUserIDIncludingRevoked(txCtx, userIDObj)
		} else {
			profileLinks, err = tx.ProfileLinks.FindByUserID(txCtx, userIDObj)
		}
		if err != nil {
			return err
		}

		// 遍历查询每个档案信息
		results = make([]*ProfileLinkResult, 0, len(profileLinks))
		for _, g := range profileLinks {
			if g == nil {
				continue
			}
			profile, err := tx.Profiles.FindByID(txCtx, g.Profile)
			if err != nil {
				return err
			}
			results = append(results, toProfileLinkResult(g, profile))
		}

		return nil
	})

	return results, err
}

// ListProfileLinksByProfileID 列出档案的所有关系用户
func (s *profileLinkQueryApplicationService) ListProfileLinksByProfileID(ctx context.Context, profileID string) ([]*ProfileLinkResult, error) {
	return s.listProfileLinksByProfileID(ctx, profileID, false)
}

// ListProfileLinksByProfileIDIncludingRevoked 列出档案的所有关系用户（包含已撤销）
func (s *profileLinkQueryApplicationService) ListProfileLinksByProfileIDIncludingRevoked(ctx context.Context, profileID string) ([]*ProfileLinkResult, error) {
	return s.listProfileLinksByProfileID(ctx, profileID, true)
}

func (s *profileLinkQueryApplicationService) listProfileLinksByProfileID(ctx context.Context, profileID string, includeRevoked bool) ([]*ProfileLinkResult, error) {
	var results []*ProfileLinkResult

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {

		profileIDObj, err := parseProfileID(profileID)
		if err != nil {
			return err
		}

		var profileLinks []*domain.ProfileLink
		if includeRevoked {
			profileLinks, err = tx.ProfileLinks.FindByProfileIDIncludingRevoked(txCtx, profileIDObj)
		} else {
			profileLinks, err = tx.ProfileLinks.FindByProfileID(txCtx, profileIDObj)
		}
		if err != nil {
			return err
		}

		// 查询档案信息（所有档案关系共享同一个档案）
		profile, err := tx.Profiles.FindByID(txCtx, profileIDObj)
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
