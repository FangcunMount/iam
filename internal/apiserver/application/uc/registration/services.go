package registration

import (
	"context"

	childapp "github.com/FangcunMount/iam/internal/apiserver/application/uc/child"
	guardapp "github.com/FangcunMount/iam/internal/apiserver/application/uc/guardianship"
)

// ChildRegistrationService 负责需要跨 child/guardianship 聚合的注册用例。
type ChildRegistrationService interface {
	RegisterChildWithGuardian(ctx context.Context, dto RegisterChildWithGuardianDTO) (*RegisterChildWithGuardianResult, error)
}

// RegisterChildWithGuardianDTO 同时注册儿童并建立监护关系。
type RegisterChildWithGuardianDTO struct {
	UserID   string
	Name     string
	Gender   uint8
	Birthday string
	IDCard   string
	Height   *uint32
	Weight   *uint32
	Relation string
}

// RegisterChildWithGuardianResult 聚合 child 和 guardianship 的返回结果。
type RegisterChildWithGuardianResult struct {
	Child        *childapp.ChildResult
	Guardianship *guardapp.GuardianshipResult
}
