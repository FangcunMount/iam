package policy

import (
	"context"

	authzDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz"
	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	roleDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type PolicyQueryService struct {
	policyRepo      policyDomain.Repository
	permissionStore RolePermissionStore
	roleRepo        roleDomain.Repository
}

func NewPolicyQueryService(
	policyRepo policyDomain.Repository,
	permissionStore RolePermissionStore,
	roleRepo roleDomain.Repository,
) *PolicyQueryService {
	return &PolicyQueryService{
		policyRepo:      policyRepo,
		permissionStore: permissionStore,
		roleRepo:        roleRepo,
	}
}

func (s *PolicyQueryService) GetPermissionsForRole(
	ctx context.Context,
	query RolePermissionsQuery,
) ([]authzDomain.Permission, error) {
	// 1. 获取角色信息
	role, err := s.roleRepo.FindByID(ctx, meta.FromUint64(query.RoleID))
	if err != nil {
		return nil, err
	}

	// 2. 查询策略规则
	return s.permissionStore.PermissionsForRole(ctx, role.Name, query.TenantID)
}

func (s *PolicyQueryService) GetCurrentVersion(
	ctx context.Context,
	query CurrentVersionQuery,
) (*policyDomain.PolicyVersion, error) {
	return s.policyRepo.GetCurrent(ctx, query.TenantID)
}

var _ PermissionReader = (*PolicyQueryService)(nil)
