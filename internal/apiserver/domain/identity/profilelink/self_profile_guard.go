package profilelink

import (
	"context"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// SelfProfileGuard 保护 self profile 的领域不变量。
type SelfProfileGuard struct {
	links Repository
}

// SelfProfileGuarder 保护 self profile 唯一性。
var _ SelfProfileGuarder = (*SelfProfileGuard)(nil)

// NewSelfProfileGuard 创建 self profile guard。
func NewSelfProfileGuard(links Repository) *SelfProfileGuard {
	return &SelfProfileGuard{links: links}
}

// EnsureCanCreateSelf 确保当前 User 还没有 active self ProfileLink。
func (g *SelfProfileGuard) EnsureCanCreateSelf(ctx context.Context, userID meta.ID) error {
	hasSelf, err := g.HasActiveSelfProfile(ctx, userID)
	if err != nil {
		return err
	}
	if hasSelf {
		return perrors.WithCode(code.ErrIdentityProfileLinkExists, "active self profile link already exists")
	}
	return nil
}

// HasActiveSelfProfile 判断当前 User 是否已有 active self ProfileLink。
func (g *SelfProfileGuard) HasActiveSelfProfile(ctx context.Context, userID meta.ID) (bool, error) {
	links, err := g.links.FindByUserID(ctx, userID)
	if err != nil {
		return false, perrors.WrapC(err, code.ErrDatabase, "find user profile links failed")
	}
	for _, link := range links {
		if link == nil {
			continue
		}
		if link.User == userID && link.Type == TypeSelf && link.Rel == RelSelf && link.IsActive() {
			return true, nil
		}
	}
	return false, nil
}
