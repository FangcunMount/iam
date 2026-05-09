package onboarding

import (
	"context"
	"strings"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type LoginIdentityEnsureStatus string

const (
	LoginIdentityCreated LoginIdentityEnsureStatus = "created"
	LoginIdentityReused  LoginIdentityEnsureStatus = "reused"
)

type LoginIdentityEnsureResult struct {
	Identity *loginidentity.LoginIdentity
	Status   LoginIdentityEnsureStatus
}

func (r LoginIdentityEnsureResult) IsNewLoginIdentity() bool {
	return r.Status == LoginIdentityCreated
}

type loginIdentityEnsurer struct{}

func newLoginIdentityEnsurer() *loginIdentityEnsurer { return &loginIdentityEnsurer{} }

func (e *loginIdentityEnsurer) Ensure(
	ctx context.Context,
	repo loginidentity.Repository,
	req *NormalizedOnboardingRequest,
	userID meta.ID,
) (*LoginIdentityEnsureResult, error) {
	identity, err := e.toDomainIdentity(req, userID)
	if err != nil {
		return nil, err
	}
	existing, err := repo.GetByProviderKey(ctx, identity.Provider, identity.Realm, identity.Identifier)
	if err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to query login identity: %v", err)
	}
	if existing != nil {
		return &LoginIdentityEnsureResult{Identity: existing, Status: LoginIdentityReused}, nil
	}
	if err := repo.Create(ctx, identity); err != nil {
		return nil, perrors.WithCode(code.ErrDatabase, "failed to save login identity: %v", err)
	}
	return &LoginIdentityEnsureResult{Identity: identity, Status: LoginIdentityCreated}, nil
}

func (e *loginIdentityEnsurer) toDomainIdentity(
	req *NormalizedOnboardingRequest,
	userID meta.ID,
) (*loginidentity.LoginIdentity, error) {
	switch req.Plan.Scenario {
	case OnboardOperaPassword:
		key := loginidentity.UsernameProviderKey(req.ScopedTenantID, usernameIdentifier(req))
		return loginidentity.NewUsernameIdentity(userID, key.Realm, key.Identifier, loginidentity.WithProfile(req.Profile), loginidentity.WithMeta(req.Meta))
	case OnboardMockConsumerPassword:
		key := loginidentity.MockConsumerProviderKey(usernameIdentifier(req))
		return loginidentity.NewUsernameIdentity(userID, key.Realm, key.Identifier, loginidentity.WithProfile(req.Profile), loginidentity.WithMeta(req.Meta))
	case OnboardPhoneOTP:
		key := loginidentity.PhoneProviderKey(req.Phone.String())
		return loginidentity.NewPhoneIdentity(userID, key.Identifier, loginidentity.WithProfile(req.Profile), loginidentity.WithMeta(req.Meta))
	case OnboardWechatMini:
		key := loginidentity.WechatMinipProviderKey(
			valueOfStringPtr(req.WechatAppID),
			valueOfStringPtr(req.WechatOpenID),
			valueOfStringPtr(req.WechatUnionID),
		)
		return loginidentity.NewWechatMinipIdentity(
			userID,
			key.Realm,
			key.Identifier,
			key.GlobalIdentifier,
			loginidentity.WithProfile(req.Profile),
			loginidentity.WithMeta(req.Meta),
		)
	case OnboardWecom:
		key := loginidentity.WecomProviderKey(
			valueOfStringPtr(req.WecomCorpID),
			valueOfStringPtr(req.WecomUserID),
		)
		return loginidentity.NewWecomIdentity(
			userID,
			key.Realm,
			key.Identifier,
			loginidentity.WithProfile(req.Profile),
			loginidentity.WithMeta(req.Meta),
		)
	default:
		return nil, perrors.WithCode(code.ErrInvalidArgument, "unsupported onboarding scenario: %s", req.Plan.Scenario)
	}
}

func usernameIdentifier(req *NormalizedOnboardingRequest) string {
	if req != nil && strings.TrimSpace(req.OperaLoginID) != "" {
		return strings.TrimSpace(req.OperaLoginID)
	}
	if req != nil && !req.Email.IsEmpty() {
		return strings.TrimSpace(req.Email.String())
	}
	if req != nil && !req.Phone.IsEmpty() {
		return strings.TrimSpace(req.Phone.String())
	}
	return ""
}

func loginIdentityLookupKey(req *NormalizedOnboardingRequest) (loginidentity.ProviderKey, bool) {
	if req == nil {
		return loginidentity.ProviderKey{}, false
	}
	var key loginidentity.ProviderKey
	switch req.Plan.Scenario {
	case OnboardOperaPassword:
		key = loginidentity.UsernameProviderKey(req.ScopedTenantID, usernameIdentifier(req))
	case OnboardMockConsumerPassword:
		key = loginidentity.MockConsumerProviderKey(usernameIdentifier(req))
	case OnboardPhoneOTP:
		key = loginidentity.PhoneProviderKey(req.Phone.String())
	case OnboardWechatMini:
		key = loginidentity.WechatMinipProviderKey(
			valueOfStringPtr(req.WechatAppID),
			valueOfStringPtr(req.WechatOpenID),
			valueOfStringPtr(req.WechatUnionID),
		)
	case OnboardWecom:
		key = loginidentity.WecomProviderKey(
			valueOfStringPtr(req.WecomCorpID),
			valueOfStringPtr(req.WecomUserID),
		)
	default:
		return loginidentity.ProviderKey{}, false
	}
	return key, key.IsValid()
}
