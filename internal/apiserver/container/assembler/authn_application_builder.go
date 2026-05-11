package assembler

import (
	"fmt"
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	credentialApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/credential"
	jwksApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/jwks"
	linkingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/proof"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/login/reauth"
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
	m.loginIdentityOnboarder = onboardingApp.NewLoginIdentityOnboarder(
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
	challengeService := challengeApp.NewService(infra.challengeRepo, challengeApp.SMSOTPDelivery{
		Gate:     infra.otpRedis,
		SMS:      smsSender,
		TTL:      smsOptions.LoginOTPTTL,
		Cooldown: smsOptions.LoginOTPSendCooldown,
		CodeLen:  smsOptions.LoginOTPCodeLength,
	}, challengeApp.NewCreator(infra.challengeRepo), challengeApp.NewVerifier(infra.challengeRepo))
	m.loginIdentityLinking = linkingApp.NewService(linkingApp.Dependencies{
		LoginIdentities: infra.loginIdentityStore,
		Challenge:       challengeService,
		IDP:             infra.idp,
		WechatApps:      infra.wechatAppQuerier,
		SecretVault:     infra.secretVault,
		WecomAgentID:    idpOptions.WeCom.AgentID,
	})

	tokenService := token.NewTokenApplicationService(token.TokenApplicationDependencies{
		AccessTokenCodec: infra.jwtGenerator,
		TokenStore:       infra.tokenStore,
		SessionManager:   domain.sessionManager,
		AccessChecker:    infra.accessChecker,
		ClaimMapper:      infra.jwtGenerator.ClaimMapper(),
		AccessTTL:        domain.accessTTL,
		RefreshTTL:       domain.refreshTTL,
	})

	authenticator := authentication.NewAuthenticator(
		authentication.NewPasswordAuthStrategyWithLoginIdentity(infra.credentialRepo, infra.loginIdentityRepo, hasher),
		authentication.NewPhoneOTPAuthStrategyWithLoginIdentity(infra.loginIdentityRepo, challengeService),
		authentication.NewOAuthWechatMinipAuthStrategyWithLoginIdentity(infra.loginIdentityRepo, infra.idp),
		authentication.NewOAuthWeChatComAuthStrategyWithLoginIdentity(infra.loginIdentityRepo, infra.idp),
	)

	loginService, err := login.NewLoginApplicationService(login.Dependencies{
		TokenService:       tokenService,
		Authenticator:      authenticator,
		MethodRegistry:     method.DefaultSelector(),
		ReAuthenticator:    reauth.NewTokenReAuthenticator(tokenService),
		CredentialRecorder: credentialApp.NewRecorder(credentialApp.Dependencies{Credentials: infra.credentialRepo}),
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

	m.tokenService = tokenService
	m.sessionService = sessionApp.NewSessionApplicationService(domain.sessionManager)

	logger := log.New(log.NewOptions())
	m.keyManagementApp = jwksApp.NewKeyManagementAppService(keyset.NewApplicationKeyManager(infra.keyManager), logger)
	m.keyPublishApp = jwksApp.NewKeyPublishAppService(keyset.NewApplicationKeyPublisher(infra.keySetBuilder), logger)
	m.keyRotationApp = jwksApp.NewKeyRotationAppService(keyset.NewApplicationKeyRotator(infra.keyRotation), logger)
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
