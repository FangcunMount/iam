package version

import (
	"context"

	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
)

type VersionService struct {
	versionRepo policyDomain.Repository
}

func NewVersionService(versionRepo policyDomain.Repository) *VersionService {
	return &VersionService{
		versionRepo: versionRepo,
	}
}

func (s *VersionService) GetOrCreateVersion(ctx context.Context, tenantID string) (*policyDomain.PolicyVersion, error) {
	return s.versionRepo.GetOrCreate(ctx, tenantID)
}

func (s *VersionService) IncrementVersion(ctx context.Context, tenantID string, changedBy, reason string) (*policyDomain.PolicyVersion, error) {
	return s.versionRepo.Increment(ctx, tenantID, changedBy, reason)
}

func (s *VersionService) GetCurrentVersion(ctx context.Context, tenantID string) (*policyDomain.PolicyVersion, error) {
	return s.versionRepo.GetCurrent(ctx, tenantID)
}
