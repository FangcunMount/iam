package linking

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// LinkWecomCommand 是企业微信登录身份绑定命令。
type LinkWecomCommand struct {
	UserID meta.ID
	CorpID string
	Code   string
}

// LinkWecom 通过企业微信外部认证结果为当前用户绑定企业微信身份。
func (s *service) LinkWecom(ctx context.Context, cmd LinkWecomCommand) (*LinkResult, error) {
	if err := requireUserID(cmd.UserID); err != nil {
		return nil, err
	}
	corpID := strings.TrimSpace(cmd.CorpID)
	authCode := strings.TrimSpace(cmd.Code)
	agentID := strings.TrimSpace(s.deps.WecomAgentID)
	if corpID == "" || authCode == "" || agentID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "corp_id, code and wecom agent_id are required")
	}
	corpSecret, err := s.appSecret(ctx, corpID, "wecom")
	if err != nil {
		return nil, err
	}
	_, userIDInWecom, err := s.idp().ExchangeWecomCode(ctx, corpID, agentID, corpSecret, authCode)
	if err != nil {
		return nil, perrors.WithCode(code.ErrInvalidCredential, "failed to exchange wecom code: %v", err)
	}
	key := loginidentity.WecomProviderKey(corpID, userIDInWecom)
	return s.ensureProviderKey(ctx, cmd.UserID, key, func() (*loginidentity.LoginIdentity, error) {
		return loginidentity.NewBuilder(cmd.UserID).
			FromProviderKey(key).
			WithVerifiedAt(s.now()).
			Build()
	})
}
