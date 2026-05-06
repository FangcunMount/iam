package profilelink

import (
	"context"

	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ================== Domain Capability Interfaces (Driving Ports) ==================
// 这些接口由领域层（领域能力）实现，供应用层调用
// 按照功能职责拆分，遵循接口隔离原则

// Linker 建立和撤销档案关系的领域能力。
type Linker interface {
	// Link 建立档案关系。
	Link(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error)
	// LinkSelf 建立本人档案关系。
	LinkSelf(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error)
	// LinkRelation 建立普通档案关系。
	LinkRelation(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error)
	// Revoke 撤销档案关系。
	Revoke(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error)
}

// SelfProfileGuarder 保护 self profile 唯一性。
type SelfProfileGuarder interface {
	// EnsureCanCreateSelf 确保可以创建本人档案。
	EnsureCanCreateSelf(ctx context.Context, userID meta.ID) error
	// HasActiveSelfProfile 判断是否存在有效的本人档案。
	HasActiveSelfProfile(ctx context.Context, userID meta.ID) (bool, error)
}
