package profile

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// ============================================
// ==== MyProfiles 当前用户档案访问用例 =====
// ============================================

func (s *myProfiles) List(ctx context.Context, userID string) ([]*ProfileResult, error) {
	var results []*ProfileResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		userIDObj, err := parseProfileAccessUserID(userID)
		if err != nil {
			return err
		}
		profileLinks, err := tx.ProfileLinks.FindByUserID(txCtx, userIDObj)
		if err != nil {
			return err
		}
		results = make([]*ProfileResult, 0, len(profileLinks))
		for _, profileLink := range profileLinks {
			if profileLink == nil {
				continue
			}
			profile, err := tx.Profiles.FindByID(txCtx, profileLink.Profile)
			if err != nil {
				return err
			}
			results = append(results, toProfileResult(profile))
		}
		return nil
	})
	return results, err
}

func (s *myProfiles) Get(ctx context.Context, userID string, profileID string) (*ProfileResult, error) {
	if err := s.ensureActiveProfileLinkAccess(ctx, userID, profileID); err != nil {
		return nil, err
	}
	return NewDirectory(s.uow).GetByID(ctx, profileID)
}

func (s *myProfiles) Patch(ctx context.Context, dto PatchMyProfileDTO) (*ProfileResult, error) {
	if err := s.ensureActiveProfileLinkAccess(ctx, dto.UserID, dto.ProfileID); err != nil {
		return nil, err
	}

	profile := NewEditor(s.uow)
	if dto.LegalName != nil && strings.TrimSpace(*dto.LegalName) != "" {
		if err := profile.Rename(ctx, dto.ProfileID, strings.TrimSpace(*dto.LegalName)); err != nil {
			return nil, err
		}
	}

	if dto.Gender != nil || dto.Birthday != nil {
		profileDTO := UpdateProfileDTO{ProfileID: dto.ProfileID}
		if dto.Gender != nil {
			profileDTO.Gender = *dto.Gender
		}
		if dto.Birthday != nil {
			profileDTO.Birthday = strings.TrimSpace(*dto.Birthday)
		}
		if err := profile.UpdateProfile(ctx, profileDTO); err != nil {
			return nil, err
		}
	}

	if dto.Height != nil || dto.Weight != nil {
		measurementDTO := UpdateHeightWeightDTO{ProfileID: dto.ProfileID}
		if dto.Height != nil {
			measurementDTO.Height = *dto.Height
		}
		if dto.Weight != nil {
			measurementDTO.Weight = *dto.Weight
		}
		if err := profile.UpdateHeightWeight(ctx, measurementDTO); err != nil {
			return nil, err
		}
	}

	return NewDirectory(s.uow).GetByID(ctx, dto.ProfileID)
}

func (s *myProfiles) ensureActiveProfileLinkAccess(ctx context.Context, userID string, profileID string) error {
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		userIDObj, err := parseProfileAccessUserID(userID)
		if err != nil {
			return err
		}
		profileIDObj, err := parseProfileID(profileID)
		if err != nil {
			return err
		}
		profileLink, err := tx.ProfileLinks.FindByUserIDAndProfileID(txCtx, userIDObj, profileIDObj)
		if err != nil {
			return err
		}
		if profileLink == nil {
			return perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
		}
		return nil
	})
	if err != nil {
		return perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
	}
	return nil
}
