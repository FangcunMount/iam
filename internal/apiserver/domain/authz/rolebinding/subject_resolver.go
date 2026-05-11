package rolebinding

import (
	"context"
	"fmt"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authz/tenant"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
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

func IsUnsupportedSubjectType(err error) bool {
	var target UnsupportedSubjectTypeError
	return errors.As(err, &target)
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

type UserSubjectResolver struct {
	users userDomain.Repository
}

func NewUserSubjectResolver(users userDomain.Repository) *UserSubjectResolver {
	return &UserSubjectResolver{users: users}
}

func (r *UserSubjectResolver) Supports(subjectType subject.Type) bool {
	return subjectType == subject.TypeUser
}

func (r *UserSubjectResolver) Resolve(ctx context.Context, sub subject.Ref, _ tenant.ID) error {
	if r == nil || r.users == nil {
		return errors.WithCode(code.ErrInternalServerError, "用户仓储未配置")
	}
	if sub.ID.IsZero() {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID格式错误")
	}
	userExists, err := r.users.FindByID(ctx, sub.ID)
	if err != nil {
		if errors.IsCode(err, code.ErrUserNotFound) {
			return errors.WithCode(code.ErrUserNotFound, "用户不存在")
		}
		return errors.Wrap(err, "检查用户存在性失败")
	}
	if userExists == nil {
		return errors.WithCode(code.ErrUserNotFound, "用户不存在")
	}
	return nil
}
