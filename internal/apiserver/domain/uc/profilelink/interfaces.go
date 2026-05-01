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
	Establish(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error)
	Revoke(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error)
}
