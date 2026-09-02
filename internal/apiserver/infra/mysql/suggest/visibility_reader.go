package suggest

import (
	"context"
	"fmt"

	appquery "github.com/FangcunMount/iam/v3/internal/apiserver/application/suggest/queryprofile"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/suggest/visibility"
	"gorm.io/gorm"
)

const visibleProfilesByCreatorSQL = `
SELECT id FROM profiles
WHERE deleted_at IS NULL AND created_by = ?
`

// VisibilityReader 按档案创建人解析 operating 用户可见的 ProfileID。
type VisibilityReader struct {
	db *gorm.DB
}

// NewVisibilityReader 创建 reader。
func NewVisibilityReader(db *gorm.DB) *VisibilityReader {
	return &VisibilityReader{db: db}
}

// VisibleProfileIDs 实现 queryprofile.VisibilityReader。
func (r *VisibilityReader) VisibleProfileIDs(ctx context.Context, principal visibility.Principal) ([]int64, error) {
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

var _ appquery.VisibilityReader = (*VisibilityReader)(nil)
