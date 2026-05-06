package profile

import (
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/identity/input"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ============= DTO 转换辅助函数 =============

// parseProfileID 解析档案ID字符串
func parseProfileID(profileID string) (meta.ID, error) {
	return input.ParseProfileID(profileID)
}

func parseProfileAccessUserID(userID string) (meta.ID, error) {
	return input.ParseUserID(userID)
}

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
		Height:   input.HeightCm(profile.Height),    // tenths of cm -> cm
		Weight:   input.WeightGrams(profile.Weight), // tenths of kg -> grams (1kg=1000g, 0.1kg=100g)
	}
}

// toProfileResults 将领域实体列表转换为 DTO 列表
func toProfileResults(profiles []*domain.Profile) []*ProfileResult {
	results := make([]*ProfileResult, 0, len(profiles))
	for _, profile := range profiles {
		if profile != nil {
			results = append(results, toProfileResult(profile))
		}
	}
	return results
}
