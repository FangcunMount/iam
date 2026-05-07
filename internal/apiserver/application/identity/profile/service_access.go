package profile

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ============================================
// ==== MyProfiles 当前用户档案访问用例 =====
// ============================================

// List 列出当前用户相关的所有档案及其关系。
func (s *myProfiles) List(ctx context.Context, userID meta.ID) ([]*ProfileResult, error) {
	var results []*ProfileResult
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		profileLinks, err := tx.ProfileLinks.FindByUserID(txCtx, userID)
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

// Get 获取当前用户与指定档案的关系和档案信息。
func (s *myProfiles) Get(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileResult, error) {
	if err := s.ensureActiveProfileLinkAccess(ctx, userID, profileID); err != nil {
		return nil, err
	}
	return NewDirectory(s.uow).GetByID(ctx, profileID)
}

// Patch 更新当前用户与指定档案的关系和/或档案信息。
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

		changed := false
		if dto.LegalName != nil && strings.TrimSpace(*dto.LegalName) != "" {
			name := strings.TrimSpace(*dto.LegalName)
			if err := profile.Rename(name); err != nil {
				return err
			}
			changed = true
		}

		if dto.Gender != nil || dto.Birthday != nil {
			gender := meta.NewGender(0)
			if dto.Gender != nil {
				gender = meta.NewGender(*dto.Gender)
			}
			birthday := meta.NewBirthday("")
			if dto.Birthday != nil {
				birthday = meta.NewBirthday(strings.TrimSpace(*dto.Birthday))
			}
			profile.UpdateProfile(gender, birthday)
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

// ensureActiveProfileLinkAccess 确保用户与档案之间存在有效的关联关系，否则返回权限错误。
func (s *myProfiles) ensureActiveProfileLinkAccess(ctx context.Context, userID meta.ID, profileID meta.ID) error {
	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		_, err := accessibleProfileIDInTx(txCtx, tx, userID, profileID)
		return err
	})
	if err != nil {
		return perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
	}
	return nil
}

// accessibleProfileIDInTx 在事务内检查用户与档案之间的关联关系，如果存在则返回档案ID，否则返回权限错误。
func accessibleProfileIDInTx(txCtx context.Context, tx uow.TxRepositories, userID meta.ID, profileID meta.ID) (meta.ID, error) {
	profileLink, err := tx.ProfileLinks.FindByUserIDAndProfileID(txCtx, userID, profileID)
	if err != nil {
		return 0, perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
	}
	if profileLink == nil {
		return 0, perrors.WithCode(code.ErrPermissionDenied, "you are not linked to this profile")
	}
	return profileID, nil
}
