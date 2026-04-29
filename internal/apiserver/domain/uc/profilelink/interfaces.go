package profilelink

import (
	"context"

	"github.com/FangcunMount/iam/internal/pkg/meta"
)

// ================== Domain Service Interfaces (Driving Ports) ==================
// 这些接口由领域层（领域服务）实现，供应用层调用
// 按照功能职责拆分，遵循接口隔离原则

// Manager 档案关系管理领域服务接口
// 负责档案关系建立和撤销相关的领域逻辑
type Manager interface {
	CreateProfileLink(ctx context.Context, userID meta.ID, profileID meta.ID, relation Relation) (*ProfileLink, error)
	RemoveProfileLink(ctx context.Context, userID meta.ID, profileID meta.ID) (*ProfileLink, error)
}

// RegisterProfileWithProfileLinkParams 同时注册档案和档案关系的参数
type RegisterProfileWithProfileLinkParams struct {
	Name     string
	Gender   meta.Gender
	Birthday meta.Birthday
	IDCard   meta.IDCard  // 可选
	Height   *meta.Height // 可选
	Weight   *meta.Weight // 可选
	UserID   meta.ID      // 关系用户ID
	Relation Relation     // 档案关系
}
