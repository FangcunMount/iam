package user

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	domain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// ============= DTO 转换辅助函数 =============

// toUserResult 将领域实体转换为 DTO
func toUserResult(user *domain.User) *UserResult {
	if user == nil {
		return nil
	}

	return &UserResult{
		ID:       user.ID.String(),
		Name:     user.Name,
		Nickname: user.Nickname,
		Phone:    user.Phone.String(),
		Email:    user.Email.String(),
		Status:   user.Status,
	}
}

func isUserNotFound(err error) bool {
	return perrors.IsCode(err, code.ErrUserNotFound)
}
