package profile

import domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/profile"

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
