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

// 确保 ProfileLinker 实现了 Linker 接口
var _ Linker = (*ProfileLinker)(nil)

// NewLinker 创建 profile linker
func NewLinker(links Repository) *ProfileLinker {
	return &ProfileLinker{
		links: links,
		now:   time.Now,
	}
}

// newLinkerWithClock 创建带 clock 的 profile linker，方便单元测试
func newLinkerWithClock(links Repository, now func() time.Time) *ProfileLinker {
	return &ProfileLinker{
		links: links,
		now:   now,
	}
}

// Link 根据 relation 建立档案关系
func (linker *ProfileLinker) Link(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error) {
	if relation == RelSelf {
		return linker.LinkSelf(ctx, userID, profileID)
	}
	return linker.LinkRelation(ctx, userID, profileID, relation)
}

// LinkSelf 建立 User 与本人档案的 self 关系
// active self 唯一性由 SelfProfileGuard 保护，调用方应在保存前显式调用 guard。
func (linker *ProfileLinker) LinkSelf(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error) {
	return linker.link(ctx, userID, profileID, RelSelf, linker.now())
}

// LinkRelation 建立普通档案关系。
func (linker *ProfileLinker) LinkRelation(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error) {
	if relation == RelSelf {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "self relation must use LinkSelf")
	}
	return linker.link(ctx, userID, profileID, relation, linker.now())
}

// link 建立 User 与 Profile 的档案关系。
// 注意：该方法只构造领域对象，不负责持久化。
func (linker *ProfileLinker) link(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation, establishedAt time.Time) (*ProfileLink, error) {
	// 判断 relation 参数合法性
	if err := validateRelation(relation); err != nil {
		return nil, err
	}

	// User 与 Profile 已经建立关联，则无法重复 link
	if isLinked, err := linker.links.IsLinked(ctx, userID, profileID); err != nil {
		return nil, perrors.WrapC(err, code.ErrDatabase, "check profile link failed")
	} else if isLinked {
		return nil, perrors.WithCode(code.ErrIdentityProfileLinkExists, "profile link already exists")
	}

	return &ProfileLink{
		User:          userID,
		Profile:       profileID,
		Type:          TypeFromRelation(relation),
		Rel:           relation,
		EstablishedAt: establishedAt,
	}, nil
}

// validateRelation Relation 值是否合法
func validateRelation(relation Relation) error {
	if relation.IsValid() {
		return nil
	}
	return perrors.WithCode(code.ErrInvalidArgument, "unsupported profile relation: %s", relation)
}

// Revoke 撤销档案关系。
// 领域逻辑：查询档案关系 + 撤销关系
// 注意：不包括持久化，返回修改后的档案关系实体供应用层持久化
func (linker *ProfileLinker) Revoke(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error) {
	profileLink, err := linker.links.FindByUserIDAndProfileID(ctx, userID, profileID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrUserInvalid, "active profile link not found")
	}

	return linker.RevokeLink(profileLink)
}

// RevokeLink 撤销已经解析出的档案关系。
// 撤销操作由 ProfileLink.Revoke 保证幂等。
func (linker *ProfileLinker) RevokeLink(profileLink *ProfileLink) (*ProfileLink, error) {
	// 参数传递错误
	if profileLink == nil {
		return nil, perrors.WithCode(code.ErrIdentityProfileLinkNotFound, "active profile link not found")
	}

	// 撤销关系
	profileLink.Revoke(linker.now())
	return profileLink, nil
}
