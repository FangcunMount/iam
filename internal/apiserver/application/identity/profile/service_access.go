package profile

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/input"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
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
	var result *ProfileResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		profileID, err := accessibleProfileIDInTx(txCtx, tx, dto.UserID, dto.ProfileID)
		if err != nil {
			return err
		}

		profile, err := tx.Profiles.FindByID(txCtx, profileID)
		if err != nil {
			return err
		}

		validator := domain.NewValidator(tx.Profiles)
		changed := false
		if dto.LegalName != nil && strings.TrimSpace(*dto.LegalName) != "" {
			name := strings.TrimSpace(*dto.LegalName)
			if err := validator.ValidateRename(name); err != nil {
				return err
			}
			profile.Rename(name)
			changed = true
		}

		if dto.Gender != nil || dto.Birthday != nil {
			gender := input.ParseGender(0)
			if dto.Gender != nil {
				gender = input.ParseGender(*dto.Gender)
			}
			birthday := input.ParseBirthday("")
			if dto.Birthday != nil {
				birthday = input.ParseBirthday(strings.TrimSpace(*dto.Birthday))
			}
			if err := validator.ValidateUpdateProfile(gender, birthday); err != nil {
				return err
			}
			profile.UpdateProfile(gender, birthday)
			changed = true
		}

		if dto.Height != nil || dto.Weight != nil {
			height, err := input.ParseHeightCm(0)
			if err != nil {
				return err
			}
			if dto.Height != nil {
				height, err = input.ParseHeightCm(*dto.Height)
				if err != nil {
					return err
				}
			}
			weight, err := input.ParseWeightGrams(0)
			if err != nil {
				return err
			}
			if dto.Weight != nil {
				weight, err = input.ParseWeightGrams(*dto.Weight)
				if err != nil {
					return err
				}
			}
			profile.UpdateHeightWeight(height, weight)
			changed = true
		}

		if changed {
			if err := tx.Profiles.Update(txCtx, profile); err != nil {
				return err
			}
		}
		result = toProfileResult(profile)
		return nil
	})
	return result, err
}

func (s *myProfiles) ensureActiveProfileLinkAccess(ctx context.Context, userID string, profileID string) error {
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		_, err := accessibleProfileIDInTx(txCtx, tx, userID, profileID)
		return err
	})
	if err != nil {
		return perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
	}
	return nil
}

func accessibleProfileIDInTx(txCtx context.Context, tx uow.TxRepositories, userID string, profileID string) (meta.ID, error) {
	userIDObj, err := parseProfileAccessUserID(userID)
	if err != nil {
		return 0, perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
	}
	profileIDObj, err := parseProfileID(profileID)
	if err != nil {
		return 0, perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
	}
	profileLink, err := tx.ProfileLinks.FindByUserIDAndProfileID(txCtx, userIDObj, profileIDObj)
	if err != nil {
		return 0, perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
	}
	if profileLink == nil {
		return 0, perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
	}
	return profileIDObj, nil
}
