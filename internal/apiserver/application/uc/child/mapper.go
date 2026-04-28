package child

import (
	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ============= DTO 转换辅助函数 =============

// parseChildID 解析儿童ID字符串
func parseChildID(childID string) (meta.ID, error) {
	return input.ParseChildID(childID)
}

func parseChildAccessUserID(userID string) (meta.ID, error) {
	return input.ParseUserID(userID)
}

// toChildResult 将领域实体转换为 DTO
func toChildResult(child *domain.Child) *ChildResult {
	if child == nil {
		return nil
	}

	return &ChildResult{
		ID:       child.ID.String(),
		Name:     child.Name,
		IDCard:   child.IDCard.String(),
		Gender:   child.Gender.Value(),
		Birthday: child.Birthday.String(),
		Height:   input.HeightCm(child.Height),    // tenths of cm -> cm
		Weight:   input.WeightGrams(child.Weight), // tenths of kg -> grams (1kg=1000g, 0.1kg=100g)
	}
}

// toChildResults 将领域实体列表转换为 DTO 列表
func toChildResults(children []*domain.Child) []*ChildResult {
	results := make([]*ChildResult, 0, len(children))
	for _, child := range children {
		if child != nil {
			results = append(results, toChildResult(child))
		}
	}
	return results
}
