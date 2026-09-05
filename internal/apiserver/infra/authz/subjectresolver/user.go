package subjectresolver

import (
	"context"
	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/subject"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authz/tenant"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

type UserSubjectResolver struct {
	users useraccess.UserResolver
}

func NewUserSubjectResolver(users useraccess.UserResolver) *UserSubjectResolver {
	return &UserSubjectResolver{users: users}
}

func (r *UserSubjectResolver) Supports(subjectType subject.Type) bool {
	return subjectType == subject.TypeUser
}

func (r *UserSubjectResolver) Resolve(ctx context.Context, sub subject.Ref, _ tenant.ID) error {
	if r == nil || r.users == nil {
		return errors.WithCode(code.ErrInternalServerError, "Identity UserResolver 未配置")
	}
	if sub.ID.IsZero() {
		return errors.WithCode(code.ErrInvalidArgument, "主体ID格式错误")
	}
	err := r.users.ResolveUser(ctx, sub.ID)
	if err != nil {
		if errors.IsCode(err, code.ErrUserNotFound) {
			return errors.WithCode(code.ErrUserNotFound, "用户不存在")
		}
		return errors.Wrap(err, "检查用户存在性失败")
	}
	return nil
}
