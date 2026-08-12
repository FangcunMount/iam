package linking

import (
	"context"

	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

// List 列出用户仍可见的登录身份。
func (l *linker) List(ctx context.Context, userID meta.ID) ([]LoginIdentityView, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}

	// 查询用户登录身份。
	identities, err := l.repo().ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 构建登录身份视图。
	out := make([]LoginIdentityView, 0, len(identities))
	for _, identity := range identities {
		if identity == nil || identity.Status == loginidentity.StatusDeleted {
			continue
		}
		out = append(out, toView(identity))
	}
	return out, nil
}

// toView 构建登录身份视图。
func toView(identity *loginidentity.LoginIdentity) LoginIdentityView {
	return LoginIdentityView{
		ID:               identity.ID,
		UserID:           identity.UserID,
		Provider:         identity.Provider,
		Realm:            identity.Realm,
		Identifier:       identity.Identifier,
		GlobalIdentifier: identity.GlobalIdentifier,
		Status:           identity.Status,
		VerifiedAt:       identity.VerifiedAt,
		LinkedAt:         identity.LinkedAt,
	}
}
