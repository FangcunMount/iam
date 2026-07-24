package profile

import (
	"time"

	appProfileLink "github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/profilelink"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	profiledomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	profileLinkDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profilelink"
)

// ============= DTO 转换辅助函数 =============

// toProfileResult 将领域实体转换为 DTO
func toProfileResult(profile *domain.Profile) *ProfileResult {
	if profile == nil {
		return nil
	}

	return &ProfileResult{
		ID:       profile.ID.String(),
		Name:     profile.Name,
		IDCard:   profile.IDCard.String(),
		Gender:   profile.Gender.Value(),
		Birthday: profile.Birthday.String(),
	}
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
