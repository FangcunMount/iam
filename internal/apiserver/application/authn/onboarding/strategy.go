package onboarding

import (
	"context"
	"fmt"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	accountDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/account"
	credDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/credential"
	userDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

type onboardingStrategy interface {
	Scenario() OnboardingScenario
	AccountType(req OnboardingRequest) (accountDomain.AccountType, error)
	CredentialType() CredentialType
	CredentialRepositoryType() credDomain.CredentialType
	BuildPlan(req OnboardingRequest) (OnboardingPlan, error)
	Normalize(ctx context.Context, deps onboardingStrategyDeps, req OnboardingRequest) (OnboardingRequest, error)
	ResolveUserByAccount(
		ctx context.Context,
		resolver *userResolver,
		userRepo userDomain.Repository,
		accountRepo accountDomain.Repository,
		req *NormalizedOnboardingRequest,
	) (*UserResolveResult, bool, error)
	IssueCredential(
		ctx context.Context,
		issuer credDomain.Issuer,
		accountID meta.ID,
		creationParams *accountDomain.CreationParams,
		req *NormalizedOnboardingRequest,
	) (*credDomain.Credential, error)
}

type onboardingStrategyDeps struct {
	wechatIdentityResolver *wechatIdentityResolver
}

type onboardingStrategyRegistry struct {
	byScenario    map[OnboardingScenario]onboardingStrategy
	byAccountType map[accountDomain.AccountType]onboardingStrategy
	phoneOTP      onboardingStrategy
}

var defaultStrategies = newOnboardingStrategyRegistry(
	passwordCredentialStrategy{baseStrategy: newBaseStrategy(
		OnboardOperaPassword,
		accountDomain.TypeOpera,
		CredTypePassword,
		credDomain.CredPassword,
	)},
	wechatMiniStrategy{baseStrategy: newBaseStrategy(
		OnboardWechatMini,
		accountDomain.TypeWcMinip,
		CredTypeWechat,
		credDomain.CredOAuthWxMinip,
		withUserRepair(),
		withCredentialReuse(),
	)},
	wecomStrategy{baseStrategy: newBaseStrategy(
		OnboardWecom,
		accountDomain.TypeWcCom,
		CredTypeWecom,
		credDomain.CredOAuthWecom,
		withCredentialReuse(),
	)},
	passwordCredentialStrategy{baseStrategy: newBaseStrategy(
		OnboardMockConsumerPassword,
		accountDomain.TypeMockConsumer,
		CredTypePassword,
		credDomain.CredPassword,
	)},
	phoneOTPStrategy{},
)

func newOnboardingStrategyRegistry(strategies ...onboardingStrategy) *onboardingStrategyRegistry {
	registry := &onboardingStrategyRegistry{
		byScenario:    make(map[OnboardingScenario]onboardingStrategy, len(strategies)),
		byAccountType: make(map[accountDomain.AccountType]onboardingStrategy, len(strategies)),
	}
	for _, strategy := range strategies {
		registry.byScenario[strategy.Scenario()] = strategy
		if strategy.Scenario() == OnboardPhoneOTP {
			registry.phoneOTP = strategy
			continue
		}
		accountType, err := strategy.AccountType(OnboardingRequest{})
		if err == nil && accountType != "" {
			registry.byAccountType[accountType] = strategy
		}
	}
	return registry
}

func (r *onboardingStrategyRegistry) Select(req OnboardingRequest) (onboardingStrategy, error) {
	if req.Scenario != "" {
		strategy, ok := r.byScenario[req.Scenario]
		if !ok {
			return nil, perrors.WithCode(code.ErrInvalidArgument, "unsupported onboarding scenario: %s", req.Scenario)
		}
		return strategy, nil
	}
	if req.CredentialType == CredTypePhone && req.AccountType.Validate() && r.phoneOTP != nil {
		return r.phoneOTP, nil
	}
	strategy, ok := r.byAccountType[req.AccountType]
	if !ok {
		return nil, perrors.WithCode(
			code.ErrInvalidArgument,
			"unsupported onboarding account/credential combination: account_type=%s credential_type=%s",
			req.AccountType,
			req.CredentialType,
		)
	}
	return strategy, nil
}

type baseStrategy struct {
	scenario                 OnboardingScenario
	accountType              accountDomain.AccountType
	credentialType           CredentialType
	credentialRepositoryType credDomain.CredentialType
	allowUserRepair          bool
	allowCredentialReuse     bool
	allowCredentialRotate    bool
}

type baseStrategyOption func(*baseStrategy)

func newBaseStrategy(
	scenario OnboardingScenario,
	accountType accountDomain.AccountType,
	credentialType CredentialType,
	credentialRepositoryType credDomain.CredentialType,
	opts ...baseStrategyOption,
) baseStrategy {
	strategy := baseStrategy{
		scenario:                 scenario,
		accountType:              accountType,
		credentialType:           credentialType,
		credentialRepositoryType: credentialRepositoryType,
	}
	for _, opt := range opts {
		opt(&strategy)
	}
	return strategy
}

func withUserRepair() baseStrategyOption {
	return func(strategy *baseStrategy) {
		strategy.allowUserRepair = true
	}
}

func withCredentialReuse() baseStrategyOption {
	return func(strategy *baseStrategy) {
		strategy.allowCredentialReuse = true
	}
}

func (s baseStrategy) Scenario() OnboardingScenario {
	return s.scenario
}

func (s baseStrategy) AccountType(OnboardingRequest) (accountDomain.AccountType, error) {
	return s.accountType, nil
}

func (s baseStrategy) CredentialType() CredentialType {
	return s.credentialType
}

func (s baseStrategy) CredentialRepositoryType() credDomain.CredentialType {
	return s.credentialRepositoryType
}

func (s baseStrategy) BuildPlan(req OnboardingRequest) (OnboardingPlan, error) {
	accountType, err := s.AccountType(req)
	if err != nil {
		return OnboardingPlan{}, err
	}
	return OnboardingPlan{
		Scenario:              s.scenario,
		AccountType:           accountType,
		CredentialType:        s.credentialType,
		NeedUser:              true,
		NeedAccount:           true,
		NeedCredential:        true,
		AllowExistingUser:     true,
		AllowUserRepair:       s.allowUserRepair,
		AllowCredentialReuse:  s.allowCredentialReuse,
		AllowCredentialRotate: s.allowCredentialRotate,
	}, nil
}

func (s baseStrategy) Normalize(_ context.Context, _ onboardingStrategyDeps, req OnboardingRequest) (OnboardingRequest, error) {
	return req, nil
}

func (s baseStrategy) ResolveUserByAccount(
	context.Context,
	*userResolver,
	userDomain.Repository,
	accountDomain.Repository,
	*NormalizedOnboardingRequest,
) (*UserResolveResult, bool, error) {
	return nil, false, nil
}

func (s baseStrategy) IssueCredential(
	context.Context,
	credDomain.Issuer,
	meta.ID,
	*accountDomain.CreationParams,
	*NormalizedOnboardingRequest,
) (*credDomain.Credential, error) {
	return nil, perrors.WithCode(code.ErrInvalidArgument, "onboarding strategy does not implement credential issuing")
}

type passwordCredentialStrategy struct {
	baseStrategy
}

func (s passwordCredentialStrategy) IssueCredential(
	ctx context.Context,
	issuer credDomain.Issuer,
	accountID meta.ID,
	_ *accountDomain.CreationParams,
	req *NormalizedOnboardingRequest,
) (*credDomain.Credential, error) {
	if req.Password == nil || *req.Password == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "password is required")
	}
	return issuer.IssuePassword(ctx, credDomain.IssuePasswordRequest{
		AccountID:     accountID,
		PlainPassword: *req.Password,
	})
}

type wechatMiniStrategy struct {
	baseStrategy
}

func (s wechatMiniStrategy) Normalize(ctx context.Context, deps onboardingStrategyDeps, req OnboardingRequest) (OnboardingRequest, error) {
	if deps.wechatIdentityResolver == nil {
		return req, nil
	}
	identity, err := deps.wechatIdentityResolver.ResolveMiniProgram(ctx, req)
	if err != nil {
		return req, err
	}
	return prepareWechatIdentity(req, identity), nil
}

func (s wechatMiniStrategy) ResolveUserByAccount(
	ctx context.Context,
	resolver *userResolver,
	userRepo userDomain.Repository,
	accountRepo accountDomain.Repository,
	req *NormalizedOnboardingRequest,
) (*UserResolveResult, bool, error) {
	if accountRepo == nil {
		return nil, false, nil
	}

	if unionID := valueOfStringPtr(req.WechatUnionID); unionID != "" {
		account, err := accountRepo.GetByUniqueID(ctx, accountDomain.UnionID(unionID))
		if err != nil && !isRepositoryNotFound(err) {
			return nil, true, err
		}
		if account != nil {
			result, err := resolver.loadOrRepairUserForAccount(ctx, userRepo, account.UserID, req, MatchedByWechatUnionID)
			return result, true, err
		}
	}

	openID := valueOfStringPtr(req.WechatOpenID)
	appID := valueOfStringPtr(req.WechatAppID)
	if openID == "" || appID == "" {
		return nil, false, nil
	}
	externalID := accountDomain.ExternalID(fmt.Sprintf("%s@%s", openID, appID))
	account, err := accountRepo.GetByExternalIDAppId(ctx, externalID, accountDomain.AppId(appID))
	if err != nil && !isRepositoryNotFound(err) {
		return nil, true, err
	}
	if account == nil {
		return nil, false, nil
	}

	result, err := resolver.loadOrRepairUserForAccount(ctx, userRepo, account.UserID, req, MatchedByWechatOpenID)
	return result, true, err
}

func (s wechatMiniStrategy) IssueCredential(
	ctx context.Context,
	issuer credDomain.Issuer,
	accountID meta.ID,
	creationParams *accountDomain.CreationParams,
	req *NormalizedOnboardingRequest,
) (*credDomain.Credential, error) {
	if creationParams == nil || creationParams.OpenID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "openid is required for wechat credential")
	}
	idpIdentifier := creationParams.OpenID
	if creationParams.UnionID != "" {
		idpIdentifier = creationParams.UnionID
	}
	appID := ""
	if req.WechatAppID != nil {
		appID = *req.WechatAppID
	}
	return issuer.IssueWechatMinip(ctx, credDomain.IssueOAuthRequest{
		AccountID:     accountID,
		IDPIdentifier: idpIdentifier,
		AppID:         appID,
		ParamsJSON:    req.ParamsJSON,
	})
}

type wecomStrategy struct {
	baseStrategy
}

func (s wecomStrategy) IssueCredential(
	ctx context.Context,
	issuer credDomain.Issuer,
	accountID meta.ID,
	_ *accountDomain.CreationParams,
	req *NormalizedOnboardingRequest,
) (*credDomain.Credential, error) {
	if req.WecomUserID == nil || *req.WecomUserID == "" {
		return nil, perrors.WithCode(code.ErrInvalidArgument, "wecom userid is required")
	}
	appID := ""
	if req.WecomCorpID != nil {
		appID = *req.WecomCorpID
	}
	return issuer.IssueWecom(ctx, credDomain.IssueOAuthRequest{
		AccountID:     accountID,
		IDPIdentifier: *req.WecomUserID,
		AppID:         appID,
		ParamsJSON:    req.ParamsJSON,
	})
}

type phoneOTPStrategy struct{}

func (s phoneOTPStrategy) Scenario() OnboardingScenario {
	return OnboardPhoneOTP
}

func (s phoneOTPStrategy) AccountType(req OnboardingRequest) (accountDomain.AccountType, error) {
	if !req.AccountType.Validate() {
		return "", perrors.WithCode(code.ErrInvalidArgument, "account_type is required for phone_otp onboarding")
	}
	return req.AccountType, nil
}

func (s phoneOTPStrategy) CredentialType() CredentialType {
	return CredTypePhone
}

func (s phoneOTPStrategy) CredentialRepositoryType() credDomain.CredentialType {
	return credDomain.CredPhoneOTP
}

func (s phoneOTPStrategy) BuildPlan(req OnboardingRequest) (OnboardingPlan, error) {
	base := newBaseStrategy(
		OnboardPhoneOTP,
		req.AccountType,
		CredTypePhone,
		credDomain.CredPhoneOTP,
		withCredentialReuse(),
	)
	return base.BuildPlan(req)
}

func (s phoneOTPStrategy) Normalize(_ context.Context, _ onboardingStrategyDeps, req OnboardingRequest) (OnboardingRequest, error) {
	return req, nil
}

func (s phoneOTPStrategy) ResolveUserByAccount(
	context.Context,
	*userResolver,
	userDomain.Repository,
	accountDomain.Repository,
	*NormalizedOnboardingRequest,
) (*UserResolveResult, bool, error) {
	return nil, false, nil
}

func (s phoneOTPStrategy) IssueCredential(
	ctx context.Context,
	issuer credDomain.Issuer,
	accountID meta.ID,
	_ *accountDomain.CreationParams,
	req *NormalizedOnboardingRequest,
) (*credDomain.Credential, error) {
	return issuer.IssuePhoneOTP(ctx, credDomain.IssuePhoneOTPRequest{
		AccountID: accountID,
		Phone:     req.Phone,
	})
}
