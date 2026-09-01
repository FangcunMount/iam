package runtime

import (
	"context"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

type RoleRecord struct {
	ID       meta.ID
	TenantID string
	Name     string
}

type AssignmentRecord struct {
	TenantID   string
	SubjectKey string
	RoleID     meta.ID
}

type InheritanceRecord struct {
	TenantID        string
	RoleID          meta.ID
	InheritedRoleID meta.ID
}

type Dataset struct {
	Roles        []RoleRecord
	Assignments  []AssignmentRecord
	Inheritances []InheritanceRecord
	Grants       []*permissiongrant.Grant
	Resources    []*resource.Resource
	Versions     map[string]int64
}

type Source interface {
	Load(ctx context.Context) (Dataset, error)
}
