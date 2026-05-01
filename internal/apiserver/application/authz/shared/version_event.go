package shared

import (
	"context"
	"fmt"

	policyDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/policy"
	"github.com/FangcunMount/iam/v2/pkg/event"
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
