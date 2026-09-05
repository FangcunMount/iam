package authorization

import (
	"context"
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/authorization"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/pkg/tenant"
)

// RequireCatalogWrite checks the authenticated actor in the platform domain.
func (s *DecisionService) RequireCatalogWrite(ctx context.Context, actor subject.Ref, action string) error {
	request, err := domain.NewRequest(actor, tenant.PlatformID, ResourceResources, action, domain.ObjectContext{})
	if err != nil {
		return err
	}
	decision, err := s.Check(ctx, request)
	if err != nil {
		return err
	}
	if !decision.Allowed {
		return perrors.WithCode(code.ErrPermissionDenied, "platform catalog permission required")
	}
	return nil
}
