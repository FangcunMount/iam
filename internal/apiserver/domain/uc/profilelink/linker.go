package profilelink

import (
	"context"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/profile"
	"github.com/FangcunMount/iam/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/internal/pkg/code"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ProfileLinker 建立和撤销档案关系的领域能力。
type ProfileLinker struct {
	repo        Repository
	profileRepo profile.Repository
	userRepo    user.Repository
}

// 确保实现
var _ Linker = (*ProfileLinker)(nil)

// NewLinker 创建档案关系领域能力。
func NewLinker(r Repository, cr profile.Repository, ur user.Repository) *ProfileLinker {
	return &ProfileLinker{
		repo:        r,
		profileRepo: cr,
		userRepo:    ur,
	}
}

// Establish 建立档案关系。
// 领域逻辑：验证用户和档案存在性 + 验证档案关系不重复 + 创建关系实体
// 注意：不包括持久化，返回创建的档案关系实体供应用层持久化
func (s *ProfileLinker) Establish(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error) {
	// 验证档案存在
	c, err := s.profileRepo.FindByID(ctx, profileID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrDatabase, "find profile failed")
	}
	if c == nil {
		return nil, perrors.WithCode(code.ErrUserInvalid, "profile not found")
	}

	// 验证用户存在
	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrDatabase, "find user failed")
	}
	if u == nil {
		return nil, perrors.WithCode(code.ErrUserInvalid, "user not found")
	}

	// 验证档案关系不重复
	profileLinks, err := s.repo.FindByProfileID(ctx, profileID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrDatabase, "find profile links failed")
	}
	for _, g := range profileLinks {
		if g == nil {
			continue
		}
		if g.User == userID && g.IsActive() {
			return nil, perrors.WithCode(code.ErrUserInvalid, "profile link already exists")
		}
	}

	// 创建档案关系实体
	newProfileLink := &ProfileLink{
		User:          userID,
		Profile:       profileID,
		Type:          TypeFromRelation(relation),
		Rel:           relation,
		EstablishedAt: time.Now(),
	}

	// 返回创建的档案关系，由应用层持久化
	return newProfileLink, nil
}

// Revoke 撤销档案关系。
// 领域逻辑：查询档案关系 + 撤销关系
// 注意：不包括持久化，返回修改后的档案关系实体供应用层持久化
func (s *ProfileLinker) Revoke(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error) {
	// 查询档案关系
	profileLinks, err := s.repo.FindByProfileID(ctx, profileID)
	if err != nil {
		return nil, perrors.WrapC(err, code.ErrDatabase, "find profile links failed")
	}

	// 查找目标档案关系
	var target *ProfileLink
	for _, g := range profileLinks {
		if g == nil {
			continue
		}
		if g.User == userID && g.IsActive() {
			target = g
			break
		}
	}

	if target == nil {
		return nil, perrors.WithCode(code.ErrUserInvalid, "active profile link not found")
	}

	// 撤销档案关系
	target.Revoke(time.Now())

	// 返回修改后的档案关系，由应用层持久化
	return target, nil
}
