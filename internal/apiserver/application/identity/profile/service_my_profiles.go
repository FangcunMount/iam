package profile

import (
	"context"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/input"
	appProfileLink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/uow"
	profiledomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type myProfiles struct {
	uow uow.UnitOfWork
}

// NewMyProfiles 创建跨 Profile/ProfileLink 的组合建档服务。
func NewMyProfiles(uow uow.UnitOfWork) MyProfiles {
	return &myProfiles{uow: uow}
}

func (s *myProfiles) Create(ctx context.Context, currentUserID string, dto CreateMyProfileDTO) (*CreatedProfileResult, error) {
	l := logger.L(ctx)
	var result *CreatedProfileResult

	l.Debugw("创建档案并建立关系",
		"action", logger.ActionCreate,
		"resource", "profile",
		"user_id", currentUserID,
		"profile_name", dto.Name,
		"relation", dto.Relation,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		userID, err := parseMyProfileUserID(currentUserID)
		if err != nil {
			return err
		}

		newProfile, err := buildProfileEntity(txCtx, tx, profileCreationInput{
			Name:     dto.Name,
			Gender:   dto.Gender,
			Birthday: dto.Birthday,
			IDCard:   dto.IDCard,
			Height:   dto.Height,
			Weight:   dto.Weight,
		})
		if err != nil {
			return err
		}
		if err := tx.Profiles.Create(txCtx, newProfile); err != nil {
			return err
		}

		manager := profileLinkDomain.NewLinker(tx.ProfileLinks, tx.Profiles, tx.Users)
		newProfileLink, err := manager.Establish(txCtx, userID, newProfile.ID, profileLinkDomain.ParseRelation(dto.Relation))
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

func parseMyProfileUserID(userID string) (meta.ID, error) {
	return input.ParseUserID(strings.TrimSpace(userID))
}
