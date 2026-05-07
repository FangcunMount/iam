package profilelink

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ProfileLinker 建立和撤销 User -> Profile 档案关系。
type ProfileLinker struct {
	links Repository
	now   func() time.Time
}

var _ Linker = (*ProfileLinker)(nil)

// NewLinker 创建档案关系领域能力。
func NewLinker(links Repository) *ProfileLinker {
	return &ProfileLinker{
		links: links,
		now:   time.Now,
	}
}

func newLinkerWithClock(links Repository, now func() time.Time) *ProfileLinker {
	linker := NewLinker(links)
	linker.now = now
	return linker
}

// Link 根据 relation 建立档案关系。
func (l *ProfileLinker) Link(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error) {
	if relation == RelSelf {
		return l.LinkSelf(ctx, userID, profileID)
	}
	return l.LinkRelation(ctx, userID, profileID, relation)
}

// LinkSelf 建立 User 与本人档案的 self 关系。
// active self 唯一性由 SelfProfileGuard 保护，调用方应在保存前显式调用 guard。
func (l *ProfileLinker) LinkSelf(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error) {
	return l.link(ctx, userID, profileID, RelSelf)
}

// LinkRelation 建立普通档案关系。
func (l *ProfileLinker) LinkRelation(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error) {
	if relation == RelSelf {
		return l.LinkSelf(ctx, userID, profileID)
	}
	return l.link(ctx, userID, profileID, relation)
}

func (l *ProfileLinker) link(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error) {
	linked, err := l.links.IsLinked(ctx, userID, profileID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrDatabase, "check profile link failed")
	}
	if linked {
		return nil, perrors.WithCode(code.ErrUserInvalid, "profile link already exists")
	}

	return &ProfileLink{
		User:          userID,
		Profile:       profileID,
		Type:          TypeFromRelation(relation),
		Rel:           relation,
		EstablishedAt: l.now(),
	}, nil
}

// Revoke 撤销档案关系。
// 领域逻辑：查询档案关系 + 撤销关系
// 注意：不包括持久化，返回修改后的档案关系实体供应用层持久化
func (l *ProfileLinker) Revoke(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error) {
	profileLinks, err := l.links.FindByProfileID(ctx, profileID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrDatabase, "find profile links failed")
	}

	var target *ProfileLink
	for _, link := range profileLinks {
		if link == nil {
			continue
		}
		if link.User == userID && link.IsActive() {
			target = link
			break
		}
	}

	if target == nil {
		return nil, perrors.WithCode(code.ErrUserInvalid, "active profile link not found")
	}

	target.Revoke(l.now())
	return target, nil
}

// RevokeLink 撤销已经解析出的档案关系。
func (l *ProfileLinker) RevokeLink(profileLink *ProfileLink) (*ProfileLink, error) {
	if profileLink == nil || !profileLink.IsActive() {
		return nil, perrors.WithCode(code.ErrUserInvalid, "active profile link not found")
	}

	profileLink.Revoke(l.now())
	return profileLink, nil
}
