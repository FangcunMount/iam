package profilelink

import (
	"time"

	profiledomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profile"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/profilelink"
)

// ============= DTO 转换辅助函数 =============

// toProfileLinkResult 将领域实体转换为 DTO
func toProfileLinkResult(profileLink *domain.ProfileLink, profile *profiledomain.Profile) *ProfileLinkResult {
	if profileLink == nil {
		return nil
	}

	result := &ProfileLinkResult{
		ID:            profileLink.ID.Uint64(),
		UserID:        profileLink.User.String(),
		ProfileID:     profileLink.Profile.String(),
		Relation:      string(profileLink.Rel), // Relation 是 string 类型
		EstablishedAt: profileLink.EstablishedAt.Format(time.RFC3339),
	}
	if profileLink.RevokedAt != nil && !profileLink.RevokedAt.IsZero() {
		result.RevokedAt = profileLink.RevokedAt.Format(time.RFC3339)
	}

	// 添加档案信息
	if profile != nil {
		result.ProfileName = profile.Name
		result.ProfileGender = profile.Gender.Value()
		result.ProfileBirthday = profile.Birthday.String()
	}

	return result
}
