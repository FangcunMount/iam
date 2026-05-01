package user

import (
	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/uc/input"
	domain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/uc/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// ============= DTO 转换辅助函数 =============

// parseUserID 解析用户ID字符串
func parseUserID(userID string) (meta.ID, error) {
	return input.ParseUserID(userID)
}

// toUserResult 将领域实体转换为 DTO
func toUserResult(user *domain.User) *UserResult {
	if user == nil {
		return nil
	}

	return &UserResult{
		ID:     user.ID.String(),
		Name:   user.Name,
		Phone:  user.Phone.String(),
		Email:  user.Email.String(),
		IDCard: user.IDCard.String(),
		Status: user.Status,
	}
}

func isUserNotFound(err error) bool {
	return perrors.IsCode(err, code.ErrUserNotFound)
}
