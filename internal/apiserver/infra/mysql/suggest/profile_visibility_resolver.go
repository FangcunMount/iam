package suggest

import (
	"context"
	"fmt"

	appsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/application/suggest"
	domainsuggest "github.com/FangcunMount/iam/v2/internal/apiserver/domain/suggest"
	"gorm.io/gorm"
)

const visibleProfilesByCreatorSQL = `
SELECT id FROM profiles
WHERE deleted_at IS NULL AND created_by = ?
`

// ProfileVisibilityResolver 按档案创建人（过渡读模型）解析 operating 用户可见的 ProfileID。
type ProfileVisibilityResolver struct {
	db *gorm.DB
}

// NewProfileVisibilityResolver 创建 Resolver；db 为 nil 时 VisibleProfileIDs 返回空。
func NewProfileVisibilityResolver(db *gorm.DB) *ProfileVisibilityResolver {
	return &ProfileVisibilityResolver{db: db}
}

// VisibleProfileIDs 实现 appsuggest.ProfileVisibilityIDsResolver。
func (r *ProfileVisibilityResolver) VisibleProfileIDs(ctx context.Context, principal domainsuggest.OperatingPrincipal) ([]int64, error) {
	if r == nil || r.db == nil || principal.OperatorID <= 0 {
		return nil, nil
	}
	var ids []int64
	if err := r.db.WithContext(ctx).Raw(visibleProfilesByCreatorSQL, principal.OperatorID).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("suggest profile visibility query: %w", err)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			out = append(out, id)
		}
	}
	return out, nil
}

var _ appsuggest.ProfileVisibilityIDsResolver = (*ProfileVisibilityResolver)(nil)
