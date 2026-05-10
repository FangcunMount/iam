package linking

import (
	"context"
	"strings"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	idpPort "github.com/FangcunMount/iam/v2/internal/apiserver/domain/idp/wechatapp"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// Service 管理已认证用户的登录身份绑定。
type Service interface {
	// List 列出用户登录身份
	List(ctx context.Context, userID meta.ID) ([]LoginIdentityView, error)
	// SendPhoneLinkChallenge 发送手机登录身份链接验证码
	SendPhoneLinkChallenge(ctx context.Context, userID meta.ID, phone string) error
	// LinkPhone 链接手机登录身份
	LinkPhone(ctx context.Context, cmd LinkPhoneCommand) (*LinkResult, error)
	// LinkWechatMini 链接微信小程序登录身份
	LinkWechatMini(ctx context.Context, cmd LinkWechatMiniCommand) (*LinkResult, error)
	// LinkWecom 链接企业微信登录身份
	LinkWecom(ctx context.Context, cmd LinkWecomCommand) (*LinkResult, error)
	// Unlink 取消链接登录身份
	Unlink(ctx context.Context, cmd UnlinkCommand) error
}

// Dependencies 是登录身份绑定应用服务依赖。
type Dependencies struct {
	LoginIdentities loginidentity.Repository
	Challenge       challengeapp.Service
	IDP             authentication.IdentityProvider
	WechatApps      idpPort.Repository
	SecretVault     idpPort.SecretVault
	WecomAgentID    string
	Now             func() time.Time
}

// LinkResult 是绑定登录身份后的结果。
type LinkResult struct {
	Identity *loginidentity.LoginIdentity
	Reused   bool
}

type service struct {
	deps Dependencies
}

// NewService 创建登录身份绑定应用服务。
func NewService(deps Dependencies) Service {
	return &service{deps: deps}
}

// ensureProviderKey 确保提供者键可用
func (s *service) ensureProviderKey(
	ctx context.Context,
	userID meta.ID,
	key loginidentity.ProviderKey,
	build func() (*loginidentity.LoginIdentity, error),
) (*LinkResult, error) {
	if !key.IsValid() {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "invalid login identity provider key")
	}
	existing, err := s.repo().GetByProviderKey(ctx, key.Provider, key.Realm, key.Identifier)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.UserID != userID {
			return nil, perrors.WithCode(code.ErrLoginIdentityExists, "login identity already belongs to another user")
		}
		if !existing.IsActive() {
			return nil, perrors.WithCode(code.ErrLoginIdentityDisabled, "login identity is not active")
		}
		return &LinkResult{Identity: existing, Reused: true}, nil
	}
	identity, err := build()
	if err != nil {
		return nil, err
	}
	if err := s.repo().Create(ctx, identity); err != nil {
		return nil, err
	}
	return &LinkResult{Identity: identity}, nil
}

func (s *service) ensureGlobalIdentifierAvailable(ctx context.Context, userID meta.ID, key loginidentity.ProviderKey) error {
	if strings.TrimSpace(key.GlobalIdentifier) == "" {
		return nil
	}
	existing, err := s.repo().GetByGlobalIdentifier(ctx, key.Provider, key.GlobalIdentifier)
	if err != nil {
		return err
	}
	if existing != nil && existing.UserID != userID {
		return perrors.WithCode(code.ErrGlobalIdentifierExists, "global identifier already belongs to another user")
	}
	return nil
}

func (s *service) appSecret(ctx context.Context, appID, providerName string) (string, error) {
	if s.deps.WechatApps == nil || s.deps.SecretVault == nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "%s app configuration service is not available", providerName)
	}
	app, err := s.deps.WechatApps.GetByAppID(ctx, appID)
	if err != nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to query %s app: %v", providerName, err)
	}
	if app == nil || !app.IsEnabled() || app.Cred == nil || app.Cred.Auth == nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "%s app is not available", providerName)
	}
	plain, err := s.deps.SecretVault.Decrypt(ctx, app.Cred.Auth.AppSecretCipher)
	if err != nil {
		return "", perrors.WithCode(code.ErrInvalidArgument, "failed to decrypt %s app secret: %v", providerName, err)
	}
	return string(plain), nil
}

func (s *service) repo() loginidentity.Repository {
	return s.deps.LoginIdentities
}

func (s *service) idp() authentication.IdentityProvider {
	return s.deps.IDP
}

func (s *service) now() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now()
	}
	return time.Now()
}

func requireUserID(userID meta.ID) error {
	if userID.IsZero() {
		return perrors.WithCode(code.ErrInvalidArgument, "user_id is required")
	}
	return nil
}
