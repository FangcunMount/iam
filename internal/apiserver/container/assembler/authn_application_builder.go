package assembler

import (
	"context"
	"fmt"
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
	accountApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/account"
	jwksApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/proof"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/reauth"
	loginprep "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/loginprep"
	onboardingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/onboarding"
	sessionApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	smsInfra "github.com/FangcunMount/iam/v2/internal/apiserver/infra/sms"
	"github.com/FangcunMount/iam/v2/internal/apiserver/infra/token/keyset"
	apiserveroptions "github.com/FangcunMount/iam/v2/internal/apiserver/options"
)

func (m *AuthnModule) initializeApplication(
	infra *authnInfrastructureComponents,
	domain *authnDomainComponents,
	hasher authentication.PasswordHasher,
	idpOptions apiserveroptions.IDPOptions,
	smsOptions apiserveroptions.SMSOptions,
) error {
	m.accountService = accountApp.NewAccountApplicationService(infra.unitOfWork, domain.sessionManager)

	m.accountOnboarder = onboardingApp.NewAccountOnboarder(
		infra.unitOfWork,
		hasher,
		infra.idp,
		infra.userRepo,
		infra.wechatAppQuerier,
		infra.secretVault,
	)

	smsSender, err := buildLoginSMSSender(infra, smsOptions)
	if err != nil {
		return err
	}
	phoneOTP := &loginprep.PhoneOTPDeps{
		Store:    infra.otpRedis,
		Gate:     infra.otpRedis,
		SMS:      smsSender,
		TTL:      smsOptions.LoginOTPTTL,
		Cooldown: smsOptions.LoginOTPSendCooldown,
		CodeLen:  smsOptions.LoginOTPCodeLength,
	}
	m.loginPreparationService = loginprep.NewLoginPreparationService(phoneOTP)

	tokenIssuer := token.NewIssuer(
		infra.jwtGenerator,
		infra.tokenStore,
		domain.sessionManager,
		infra.jwtGenerator.ClaimMapper(),
		domain.accessTTL,
		domain.refreshTTL,
	)
	tokenRefresher := token.NewRefresher(
		tokenIssuer.SessionTokenPairIssuer(),
		infra.tokenStore,
		domain.sessionManager,
		infra.accessChecker,
		infra.jwtGenerator.ClaimMapper(),
	)
	tokenVerifier := token.NewVerifier(
		infra.jwtGenerator,
		infra.tokenStore,
		domain.sessionManager,
		infra.accessChecker,
	)

	authenticator := authentication.NewAuthenticator(
		authentication.NewPasswordAuthStrategy(infra.credentialRepo, infra.accountRepo, hasher),
		authentication.NewPhoneOTPAuthStrategy(infra.credentialRepo, infra.accountRepo, infra.otpVerifier),
		authentication.NewOAuthWechatMinipAuthStrategy(infra.credentialRepo, infra.accountRepo, infra.idp),
		authentication.NewOAuthWeChatComAuthStrategy(infra.credentialRepo, infra.accountRepo, infra.idp),
	)

	loginService, err := login.NewLoginApplicationService(login.Dependencies{
		TokenIssuer:     tokenIssuer,
		TokenRevoker:    loginTokenRevoker{access: tokenIssuer, refresh: tokenRefresher},
		Authenticator:   authenticator,
		MethodRegistry:  method.DefaultSelector(),
		ReAuthenticator: reauth.NewTokenReAuthenticator(tokenVerifier),
		ProofFactory: proof.DefaultFactory(
			infra.wechatAppQuerier,
			infra.secretVault,
			proof.WecomConfig{AgentID: idpOptions.WeCom.AgentID},
		),
	})
	if err != nil {
		return err
	}
	m.loginService = loginService

	m.tokenService = token.NewTokenApplicationService(
		tokenIssuer,
		tokenRefresher,
		tokenVerifier,
	)
	m.sessionService = sessionApp.NewSessionApplicationService(domain.sessionManager)

	logger := log.New(log.NewOptions())
	m.keyManagementApp = jwksApp.NewKeyManagementAppService(keyset.NewApplicationKeyManager(infra.keyManager), logger)
	m.keyPublishApp = jwksApp.NewKeyPublishAppService(keyset.NewApplicationKeyPublisher(infra.keySetBuilder), logger)
	m.keyRotationApp = jwksApp.NewKeyRotationAppService(keyset.NewApplicationKeyRotator(infra.keyRotation), logger)
	m.jwksSnapshotReporter = keyset.NewApplicationSnapshotReporter(infra.keySetBuilder)

	return nil
}

type loginTokenRevoker struct {
	access  token.AccessRevoker
	refresh token.RefreshRevoker
}

func (r loginTokenRevoker) RevokeAccessToken(ctx context.Context, tokenValue string) error {
	return r.access.RevokeAccessToken(ctx, tokenValue)
}

func (r loginTokenRevoker) RevokeRefreshToken(ctx context.Context, tokenValue string) error {
	return r.refresh.RevokeRefreshToken(ctx, tokenValue)
}

func buildLoginSMSSender(infra *authnInfrastructureComponents, smsOptions apiserveroptions.SMSOptions) (authentication.SMSSender, error) {
	smsProvider := strings.ToLower(strings.TrimSpace(smsOptions.Provider))
	if smsProvider == "" {
		smsProvider = "log"
	}
	switch smsProvider {
	case "log":
		return smsInfra.LogSender{}, nil
	case "mq":
		if infra.eventBus == nil {
			return nil, fmt.Errorf("sms.provider=mq requires EventBus (enable nsq.enabled and ensure EventBus is created)")
		}
		if infra.eventPublisher != nil {
			return smsInfra.NewMQLoginOTPSenderWithPublisher(infra.eventPublisher), nil
		}
		topic := strings.TrimSpace(smsOptions.MQ.Topic)
		return smsInfra.NewMQLoginOTPSender(infra.eventBus, topic), nil
	default:
		log.Warnw("unknown sms.provider, fallback to log", "sms.provider", smsProvider)
		return smsInfra.LogSender{}, nil
	}
}
