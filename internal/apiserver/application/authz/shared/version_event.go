package shared

import (
	"context"
	"fmt"

	policyDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v3/pkg/event"
)

func StagePolicyVersionChanged(ctx context.Context, stager event.Stager, tenantID string, version *policyDomain.PolicyVersion) error {
	if version == nil {
		return nil
	}
	if stager == nil {
		return fmt.Errorf("authz policy version event stager is required")
	}
	return stager.Stage(ctx, policyDomain.NewVersionChangedEvent(tenantID, version.Version))
}
