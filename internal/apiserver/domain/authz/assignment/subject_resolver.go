package assignment

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type SubjectResolver interface {
	Supports(subjectType subject.Type) bool
	Resolve(ctx context.Context, sub subject.Ref, tenantID tenant.ID) error
}

type UnsupportedSubjectTypeError struct {
	SubjectType subject.Type
}

func (e UnsupportedSubjectTypeError) Error() string {
	return fmt.Sprintf("subject type %s has no configured resolver", e.SubjectType)
}

type SubjectResolverRegistry struct {
	resolvers []SubjectResolver
}

func NewSubjectResolverRegistry(resolvers ...SubjectResolver) *SubjectResolverRegistry {
	filtered := make([]SubjectResolver, 0, len(resolvers))
	for _, resolver := range resolvers {
		if resolver != nil {
			filtered = append(filtered, resolver)
		}
	}
	return &SubjectResolverRegistry{resolvers: filtered}
}

func (r *SubjectResolverRegistry) Supports(subjectType subject.Type) bool {
	for _, resolver := range r.resolvers {
		if resolver.Supports(subjectType) {
			return true
		}
	}
	return false
}

func (r *SubjectResolverRegistry) Resolve(ctx context.Context, sub subject.Ref, tenantID tenant.ID) error {
	if sub.IsZero() {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID格式错误")
	}
	if tenantID.IsZero() {
		return errors.WithCode(code.ErrInvalidArgument, "租户ID不能为空")
	}
	for _, resolver := range r.resolvers {
		if resolver.Supports(sub.Type) {
			return resolver.Resolve(ctx, sub, tenantID)
		}
	}
	return errors.WrapC(UnsupportedSubjectTypeError{SubjectType: sub.Type}, code.ErrInvalidArgument, "主体类型 %s 未配置真实 resolver", sub.Type)
}
