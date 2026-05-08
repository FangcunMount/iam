package profile

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// MyProfiles 定义了针对当前用户的档案相关操作。
type myProfiles struct {
	uow uow.UnitOfWork
}

// 确保 myProfiles 实现了 MyProfiles 接口。
var _ MyProfiles = (*myProfiles)(nil)

// NewMyProfiles 创建跨 Profile/ProfileLink 的组合建档服务。
func NewMyProfiles(uow uow.UnitOfWork) MyProfiles {
	return &myProfiles{uow: uow}
}

// Create 创建档案并建立当前用户与档案的关系。
func (s *myProfiles) Create(ctx context.Context, currUserID meta.ID, dto CreateProfileDTO) (*CreatedProfileResult, error) {
	l := logger.L(ctx)
	var result *CreatedProfileResult

	l.Debugw("创建档案并建立关系",
		"action", logger.ActionCreate,
		"resource", "profile",
		"user_id", currUserID,
		"profile_name", dto.Name,
		"relation", dto.Relation,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		// 确保是 本人档案关系时，relation 字段必须为 "self"
		if dto.Relation != "self" {
			return perrors.WithCode(code.ErrInvalidArgument, "relation must be 'self' for my profile")
		}

		// 构建创建信息
		info, err := buildProfileCreationInfo(dto)
		if err != nil {
			return err
		}

		// 校验建档信息
		if err := checkProfileCreationInfo(txCtx, tx.Profiles, info); err != nil {
			return err
		}

		// 确保当前用户存在，以避免创建关系时关联到不存在的用户
		if err := ensureUserExists(txCtx, tx, currUserID); err != nil {
			return err
		}

		// 确保可以创建个人档案关系（self 关系，确保当前用户没有 active self 关系）
		if err := profileLinkDomain.NewSelfProfileGuard(tx.ProfileLinks).EnsureCanCreateSelf(txCtx, currUserID); err != nil {
			return err
		}

		// 创建档案记录
		newProfile, err := createProfileRecord(txCtx, tx.Profiles, info)
		if err != nil {
			return err
		}

		// 创建用户与档案的关系
		newProfileLink, err := createProfileLinkRecord(txCtx, tx.ProfileLinks, currUserID, newProfile.ID, profileLinkDomain.RelSelf)
		if err != nil {
			return err
		}

		// 构建结果
		result = &CreatedProfileResult{
			Profile:     toProfileResult(newProfile),
			ProfileLink: myProfileLinkToResult(newProfileLink, newProfile),
		}

		return nil
	})
	if err != nil {
		l.Errorw("创建个人档案失败", "err", err.Error())
		return nil, err
	}

	l.Debugw("创建个人档案成功",
		"action", logger.ActionCreate,
		"resource", "profile",
		"resource_id", result.Profile.ID,
		"profile_name", result.Profile.Name,
		"relation", dto.Relation,
		"result", logger.ResultSuccess,
	)

	return result, nil
}

// ensureMyProfileUserExists 确保当前用户存在，以避免创建关系时关联到不存在的用户。
func ensureUserExists(txCtx context.Context, tx uow.TxRepositories, userID meta.ID) error {
	user, err := tx.Users.FindByID(txCtx, userID)
	if err != nil {
		return perrors.WrapC(err, code.ErrDatabase, "find user failed")
	}
	if user == nil {
		return perrors.WithCode(code.ErrUserInvalid, "user not found")
	}
	return nil
}

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
