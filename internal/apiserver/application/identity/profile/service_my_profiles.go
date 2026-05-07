package profile

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/logger"
	appProfileLink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	profiledomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type myProfiles struct {
	uow uow.UnitOfWork
}

// NewMyProfiles 创建跨 Profile/ProfileLink 的组合建档服务。
func NewMyProfiles(uow uow.UnitOfWork) MyProfiles {
	return &myProfiles{uow: uow}
}

// Create 创建 Self Profile
func (s *myProfiles) Create(ctx context.Context, currUserID meta.ID, dto CreateMyProfileDTO) (*CreatedProfileResult, error) {
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
		newProfile, err := buildProfileEntity(profileCreationInput{
			Name:     dto.Name,
			Gender:   meta.NewGender(dto.Gender),
			Birthday: meta.NewBirthday(dto.Birthday),
		})
		if err != nil {
			return err
		}

		idCard, hasIDCard, err := optionalIDCard(dto.Name, dto.IDCard)
		if err != nil {
			return err
		}
		if hasIDCard {
			checker := profiledomain.NewIDCardUniquenessChecker(tx.Profiles)
			if err := checker.CheckIDCardUnique(txCtx, idCard); err != nil {
				return err
			}
			newProfile.IDCard = idCard
		}
		if err := tx.Profiles.Create(txCtx, newProfile); err != nil {
			return err
		}

		if err := ensureMyProfileUserExists(txCtx, tx, currUserID); err != nil {
			return err
		}

		relation := profileLinkDomain.ParseRelation(dto.Relation)
		linker := profileLinkDomain.NewLinker(tx.ProfileLinks)
		var newProfileLink *profileLinkDomain.ProfileLink
		if relation == profileLinkDomain.RelSelf {
			if err := profileLinkDomain.NewSelfProfileGuard(tx.ProfileLinks).EnsureCanCreateSelf(txCtx, currUserID); err != nil {
				return err
			}
			newProfileLink, err = linker.LinkSelf(txCtx, currUserID, newProfile.ID)
		} else {
			newProfileLink, err = linker.LinkRelation(txCtx, currUserID, newProfile.ID, relation)
		}
		if err != nil {
			return err
		}
		if err := tx.ProfileLinks.Create(txCtx, newProfileLink); err != nil {
			return err
		}

		result = &CreatedProfileResult{
			Profile:     toProfileResult(newProfile),
			ProfileLink: myProfileLinkToResult(newProfileLink, newProfile),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func ensureMyProfileUserExists(txCtx context.Context, tx uow.TxRepositories, userID meta.ID) error {
	user, err := tx.Users.FindByID(txCtx, userID)
	if err != nil {
		return perrors.WrapC(err, code.ErrDatabase, "find user failed")
	}
	if user == nil {
		return perrors.WithCode(code.ErrUserInvalid, "user not found")
	}
	return nil
}

func myProfileLinkToResult(profileLink *profileLinkDomain.ProfileLink, profile *profiledomain.Profile) *appProfileLink.ProfileLinkResult {
	if profileLink == nil {
		return nil
	}

	result := &appProfileLink.ProfileLinkResult{
		ID:            profileLink.ID.Uint64(),
		UserID:        profileLink.User.String(),
		ProfileID:     profileLink.Profile.String(),
		Relation:      string(profileLink.Rel),
		EstablishedAt: profileLink.EstablishedAt.Format(time.RFC3339),
	}
	if profileLink.RevokedAt != nil && !profileLink.RevokedAt.IsZero() {
		result.RevokedAt = profileLink.RevokedAt.Format(time.RFC3339)
	}
	if profile != nil {
		result.ProfileName = profile.Name
		result.ProfileGender = profile.Gender.Value()
		result.ProfileBirthday = profile.Birthday.String()
	}

	return result
}
