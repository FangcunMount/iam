package runtime

import (
	"context"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/scope"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/role"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/permissiongrant"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authz/resource"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
)

type RoleRecord struct {
	ManagementProtection role.ManagementProtection
	ID                   meta.ID

	Name string
}

type AssignmentRecord struct {
	ID         meta.ID
	Scope      *scope.Scope
	SubjectKey string
	RoleID     meta.ID
}

type Dataset struct {
	Roles       []RoleRecord
	Assignments []AssignmentRecord
	Grants      []*permissiongrant.Grant
	Resources   []*resource.Resource
	Version     int64
}

type Source interface {
	Load(ctx context.Context) (Dataset, error)
}
