package linking

import (
	"context"
	"time"

	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// LoginIdentityView 是当前用户已绑定登录身份的只读视图。
type LoginIdentityView struct {
	ID               meta.ID
	UserID           meta.ID
	Provider         loginidentity.Provider
	Realm            string
	Identifier       string
	GlobalIdentifier string
	Status           loginidentity.Status
	VerifiedAt       *time.Time
	LinkedAt         time.Time
}

// List 列出用户仍可见的登录身份。
func (s *service) List(ctx context.Context, userID meta.ID) ([]LoginIdentityView, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	identities, err := s.repo().ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]LoginIdentityView, 0, len(identities))
	for _, identity := range identities {
		if identity == nil || identity.Status == loginidentity.StatusDeleted {
			continue
		}
		out = append(out, toView(identity))
	}
	return out, nil
}

// toView 转换为登录身份视图
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
