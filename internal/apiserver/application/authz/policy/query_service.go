package policy

import (
	"context"

	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
	roleDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/role"
	"github.com/FangcunMount/iam/internal/pkg/meta"
)

type PolicyQueryService struct {
	policyRepo    policyDomain.Repository
	casbinAdapter policyDomain.CasbinAdapter
	roleRepo      roleDomain.Repository
}

func NewPolicyQueryService(
	policyRepo policyDomain.Repository,
	casbinAdapter policyDomain.CasbinAdapter,
	roleRepo roleDomain.Repository,
) *PolicyQueryService {
	return &PolicyQueryService{
		policyRepo:    policyRepo,
		casbinAdapter: casbinAdapter,
		roleRepo:      roleRepo,
	}
}

func (s *PolicyQueryService) GetPoliciesByRole(
	ctx context.Context,
	query policyDomain.GetPoliciesByRoleQuery,
) ([]policyDomain.PolicyRule, error) {
	// 1. 获取角色信息
	role, err := s.roleRepo.FindByID(ctx, meta.FromUint64(query.RoleID))
	if err != nil {
		return nil, err
	}

	// 2. 查询策略规则
	return s.casbinAdapter.GetPoliciesByRole(ctx, role.Key(), query.TenantID)
}

func (s *PolicyQueryService) GetCurrentVersion(
	ctx context.Context,
	query policyDomain.GetCurrentVersionQuery,
) (*policyDomain.PolicyVersion, error) {
	return s.policyRepo.GetCurrent(ctx, query.TenantID)
}
