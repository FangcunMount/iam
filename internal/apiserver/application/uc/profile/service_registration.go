package profile

import (
	"context"
	"strings"
	"time"

	"github.com/FangcunMount/component-base/pkg/logger"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	appProfileLink "github.com/FangcunMount/iam/internal/apiserver/application/uc/profilelink"
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/uow"
	profiledomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profile"
	profileLinkDomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/profilelink"
	"github.com/FangcunMount/iam/internal/pkg/meta"
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
		"action", logger.ActionRegister,
		"resource", "profile",
		"user_id", currentUserID,
		"profile_name", dto.Name,
		"relation", dto.Relation,
	)

	err := s.uow.WithinTx(ctx, func(txCtx context.Context, tx uow.TxRepositories) error {
		userID, err := parseRegistrationUserID(currentUserID)
		if err != nil {
			return err
		}

		newProfile, err := buildProfileEntity(txCtx, tx, dto)
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
			ProfileLink: registrationProfileLinkToResult(newProfileLink, newProfile),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func buildProfileEntity(txCtx context.Context, tx uow.TxRepositories, dto CreateMyProfileDTO) (*profiledomain.Profile, error) {
	name := strings.TrimSpace(dto.Name)
	validator := profiledomain.NewValidator(tx.Profiles)
	gender := input.ParseGender(dto.Gender)
	birthday := input.ParseBirthday(strings.TrimSpace(dto.Birthday))
	if err := validator.ValidateRegister(txCtx, name, gender, birthday); err != nil {
		return nil, err
	}

	options := []profiledomain.ProfileOption{
		profiledomain.WithGender(gender),
		profiledomain.WithBirthday(birthday),
	}
	if strings.TrimSpace(dto.IDCard) != "" {
		idCard, err := input.ParseIDCard(name, strings.TrimSpace(dto.IDCard))
		if err != nil {
			return nil, err
		}
		options = append(options, profiledomain.WithIDCard(idCard))
	}

	newProfile, err := profiledomain.NewProfile(name, options...)
	if err != nil {
		return nil, err
	}

	if dto.Height != nil || dto.Weight != nil {
		height := newProfile.Height
		if dto.Height != nil {
			parsedHeight, err := input.ParseHeightCm(*dto.Height)
			if err != nil {
				return nil, err
			}
			height = parsedHeight
		}

		weight := newProfile.Weight
		if dto.Weight != nil {
			parsedWeight, err := input.ParseWeightGrams(*dto.Weight)
			if err != nil {
				return nil, err
			}
			weight = parsedWeight
		}

		newProfile.UpdateHeightWeight(height, weight)
	}

	return newProfile, nil
}

func registrationProfileLinkToResult(profileLink *profileLinkDomain.ProfileLink, profile *profiledomain.Profile) *appProfileLink.ProfileLinkResult {
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

func parseRegistrationUserID(userID string) (meta.ID, error) {
	return input.ParseUserID(strings.TrimSpace(userID))
}
