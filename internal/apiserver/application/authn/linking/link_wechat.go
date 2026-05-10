package linking

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// LinkWechatMiniCommand 是微信小程序登录身份绑定命令。
type LinkWechatMiniCommand struct {
	UserID meta.ID
	AppID  string
	Code   string
}

// LinkWechatMini 通过微信 code2session 结果为当前用户绑定微信小程序身份。
func (s *service) LinkWechatMini(ctx context.Context, cmd LinkWechatMiniCommand) (*LinkResult, error) {
	if err := requireUserID(cmd.UserID); err != nil {
		return nil, err
	}
	appID := strings.TrimSpace(cmd.AppID)
	jsCode := strings.TrimSpace(cmd.Code)
	if appID == "" || jsCode == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "app_id and code are required")
	}
	appSecret, err := s.appSecret(ctx, appID, "wechat")
	if err != nil {
		return nil, err
	}
	openID, unionID, err := s.idp().ExchangeWxMinipCode(ctx, appID, appSecret, jsCode)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidCredential, "failed to exchange wechat code: %v", err)
	}
	key := loginidentity.WechatMinipProviderKey(appID, openID, unionID)
	if err := s.ensureGlobalIdentifierAvailable(ctx, cmd.UserID, key); err != nil {
		return nil, err
	}
	return s.ensureProviderKey(ctx, cmd.UserID, key, func() (*loginidentity.LoginIdentity, error) {
		return loginidentity.NewBuilder(cmd.UserID).
			FromProviderKey(key).
			WithVerifiedAt(s.now()).
			Build()
	})
}
