package guardianship

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ================== Domain Service Interfaces (Driving Ports) ==================
// 这些接口由领域层（领域服务）实现，供应用层调用
// 按照功能职责拆分，遵循接口隔离原则

// Manager 监护关系管理领域服务接口
// 负责监护关系建立和撤销相关的领域逻辑
type Manager interface {
	AddGuardian(ctx context.Context, userID meta.ID, childID meta.ID, relation Relation) (*Guardianship, error)
	RemoveGuardian(ctx context.Context, userID meta.ID, childID meta.ID) (*Guardianship, error)
}

// RegisterChildWithGuardianParams 同时注册儿童和监护关系的参数
type RegisterChildWithGuardianParams struct {
	Name     string
	Gender   meta.Gender
	Birthday meta.Birthday
	IDCard   meta.IDCard  // 可选
	Height   *meta.Height // 可选
	Weight   *meta.Weight // 可选
	UserID   meta.ID      // 监护人ID
	Relation Relation     // 监护关系
}
