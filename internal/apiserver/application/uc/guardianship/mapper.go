package guardianship

import (
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/application/uc/input"
	childdomain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/child"
	domain "github.com/FangcunMount/iam/internal/apiserver/domain/uc/guardianship"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ============= DTO 转换辅助函数 =============

// parseUserID 解析用户ID字符串
func parseUserID(userID string) (meta.ID, error) {
	return input.ParseUserID(userID)
}

// parseChildID 解析儿童ID字符串
func parseChildID(childID string) (meta.ID, error) {
	return input.ParseChildID(childID)
}

// toGuardianshipResult 将领域实体转换为 DTO
func toGuardianshipResult(guardianship *domain.Guardianship, child *childdomain.Child) *GuardianshipResult {
	if guardianship == nil {
		return nil
	}

	result := &GuardianshipResult{
		ID:            guardianship.ID.Uint64(),
		UserID:        guardianship.User.String(),
		ChildID:       guardianship.Child.String(),
		Relation:      string(guardianship.Rel), // Relation 是 string 类型
		EstablishedAt: guardianship.EstablishedAt.Format(time.RFC3339),
	}
	if guardianship.RevokedAt != nil && !guardianship.RevokedAt.IsZero() {
		result.RevokedAt = guardianship.RevokedAt.Format(time.RFC3339)
	}

	// 添加儿童信息
	if child != nil {
		result.ChildName = child.Name
		result.ChildGender = child.Gender.Value()
		result.ChildBirthday = child.Birthday.String()
	}

	return result
}
