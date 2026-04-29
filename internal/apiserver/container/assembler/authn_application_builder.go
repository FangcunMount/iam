package assembler

import (
	"fmt"
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
	accountApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/account"
	jwksApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/jwks"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/login"
	loginprep "github.com/FangcunMount/iam/internal/apiserver/application/authn/loginprep"
	onboardingApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/onboarding"
	sessionApp "github.com/FangcunMount/iam/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	smsInfra "github.com/FangcunMount/iam/internal/apiserver/infra/sms"
	"github.com/FangcunMount/iam/internal/apiserver/infra/token/keyset"
	apiserveroptions "github.com/FangcunMount/iam/internal/apiserver/options"
)

func (m *AuthnModule) initializeApplication(
	infra *authnInfrastructureComponents,
	domain *authnDomainComponents,
	hasher authentication.PasswordHasher,
	smsOptions apiserveroptions.SMSOptions,
) error {
	m.AccountService = accountApp.NewAccountApplicationService(infra.unitOfWork, domain.sessionManager)

	m.AccountOnboarder = onboardingApp.NewAccountOnboarder(
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
	m.LoginPreparationService = loginprep.NewLoginPreparationService(phoneOTP)

	tokenIssuer := token.NewIssuer(
		infra.jwtGenerator,
		infra.tokenStore,
		domain.sessionManager,
		infra.jwtGenerator.ClaimMapper(),
		domain.accessTTL,
		domain.refreshTTL,
	)
	tokenRefresher := token.NewRefresher(
		tokenIssuer,
		infra.tokenStore,
		domain.sessionManager,
		infra.accessChecker,
		infra.jwtGenerator.ClaimMapper(),
		domain.accessTTL,
		domain.refreshTTL,
	)
	tokenVerifier := token.NewVerifier(
		infra.jwtGenerator,
		infra.tokenStore,
		domain.sessionManager,
		infra.accessChecker,
	)

	m.LoginService = login.NewLoginApplicationService(
		tokenIssuer,
		tokenRefresher,
		authentication.NewAuthenticator(
			authentication.NewPasswordAuthStrategy(infra.credentialRepo, infra.accountRepo, hasher),
			authentication.NewPhoneOTPAuthStrategy(infra.credentialRepo, infra.accountRepo, infra.otpVerifier),
			authentication.NewOAuthWechatMinipAuthStrategy(infra.credentialRepo, infra.accountRepo, infra.idp),
			authentication.NewOAuthWeChatComAuthStrategy(infra.credentialRepo, infra.accountRepo, infra.idp),
		),
		tokenVerifier,
		infra.wechatAppQuerier,
		infra.secretVault,
	)

	m.TokenService = token.NewTokenApplicationService(
		tokenIssuer,
		tokenRefresher,
		tokenVerifier,
	)
	m.SessionService = sessionApp.NewSessionApplicationService(domain.sessionManager)

	logger := log.New(log.NewOptions())
	m.KeyManagementApp = jwksApp.NewKeyManagementAppService(keyset.NewApplicationKeyManager(infra.keyManager), logger)
	m.KeyPublishApp = jwksApp.NewKeyPublishAppService(keyset.NewApplicationKeyPublisher(infra.keySetBuilder), logger)
	m.KeyRotationApp = jwksApp.NewKeyRotationAppService(keyset.NewApplicationKeyRotator(infra.keyRotation), logger)
	m.jwksSnapshotReporter = keyset.NewApplicationSnapshotReporter(infra.keySetBuilder)

	return nil
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
