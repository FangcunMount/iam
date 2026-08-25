package authn

import (
	"fmt"
	"strings"

	"github.com/FangcunMount/component-base/pkg/log"
	challengeApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/challenge"
	credentialApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/credential"
	jwksApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/jwks"
	linkingApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/linking"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/session"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signin/proof"
	signupApp "github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/signup"
	"github.com/FangcunMount/iam/v3/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	credentialDomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/credential"
	smsInfra "github.com/FangcunMount/iam/v3/internal/apiserver/infra/sms"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/token/keyset"
	apiserveroptions "github.com/FangcunMount/iam/v3/internal/apiserver/options"
)

func (m *AuthnModule) initializeApplication(
	infra *authnInfrastructureComponents,
	domain *authnDomainComponents,
	hasher authentication.PasswordHasher,
	authOptions apiserveroptions.AuthOptions,
	wechatOpenOptions apiserveroptions.WechatOpenOptions,
	smsOptions apiserveroptions.SMSOptions,
) error {
	m.signupService = signupApp.NewSignupService(
		infra.unitOfWork,
		hasher,
		infra.externalResolver,
		infra.userRepo,
	)
	smsSender, err := buildLoginSMSSender(infra, smsOptions)
	if err != nil {
		return err
	}
	challengeService := challengeApp.NewService(infra.challengeRepo, challengeApp.SMSOTPDelivery{
		Gate:        infra.otpRedis,
		Quota:       infra.otpRedis,
		SMS:         smsSender,
		TTL:         smsOptions.LoginOTPTTL,
		Cooldown:    smsOptions.LoginOTPSendCooldown,
		CodeLen:     smsOptions.LoginOTPCodeLength,
		HourlyLimit: smsOptions.LoginOTPHourlyLimit,
		DailyLimit:  smsOptions.LoginOTPDailyLimit,
	}, challengeApp.NewCreator(infra.challengeRepo), challengeApp.NewVerifier(
		infra.challengeRepo,
		smsOptions.LoginOTPMaxAttempts,
	))
	m.challengeService = challengeService
	m.loginIdentityLinking = linkingApp.NewLinker(linkingApp.Dependencies{
		LoginIdentities:  infra.loginIdentityStore,
		IdentityUnlinker: infra.loginIdentityStore,
		PhoneLinkOTP:     newPhoneLinkOTPVerifierAdapter(challengeService),
		ExternalIdentity: infra.externalResolver,
	})

	tokenCapabilities := token.NewCapabilities(token.Dependencies{
		AccessTokenCodec:      infra.jwtGenerator,
		TokenStore:            infra.tokenStore,
		SessionCreator:        domain.sessionCreator,
		SessionLoader:         domain.sessionLoader,
		SessionRevoker:        domain.sessionRevoker,
		SessionExtender:       domain.sessionExtender,
		SessionRefreshExpirer: domain.sessionRefreshExpirer,
		AdmissionPolicy:       infra.admissionPolicy,
		RefreshClaimsCodec:    token.NewDefaultRefreshClaimsCodec(),
		AccessTTL:             domain.accessTTL,
	})

	authenticator := authentication.NewAuthenticator(
		authentication.NewPasswordAuthStrategyWithLoginIdentity(infra.credentialRepo, infra.loginIdentityRepo, hasher),
		newPhoneOTPAuthStrategy(infra.loginIdentityRepo, challengeService),
		authentication.NewOAuthWechatMinipAuthStrategyWithLoginIdentity(infra.loginIdentityRepo),
		authentication.NewOAuthWechatOpenAuthStrategyWithLoginIdentity(infra.loginIdentityRepo),
		authentication.NewOAuthWeChatComAuthStrategyWithLoginIdentity(infra.loginIdentityRepo),
	)
	m.startWechatOpenAuthorize = signin.NewStartWechatOpenAuthorize(challengeService, wechatOpenAuthorizeURLBuilder{})
	wechatOpenLinkStates := newWechatOpenLinkStateAdapter(challengeService, challengeService)
	m.startWechatOpenLinkAuthorize = linkingApp.NewStartWechatOpenLinkAuthorize(wechatOpenLinkStates, wechatOpenAuthorizeURLBuilder{})
	m.completeWechatOpenLink = linkingApp.NewCompleteWechatOpenLink(wechatOpenLinkStates, m.loginIdentityLinking)
	m.wechatOpenConfig = WechatOpenConfig{
		AppID:            wechatOpenOptions.AppID,
		LoginRedirectURI: wechatOpenOptions.LoginRedirectURI,
		LinkRedirectURI:  wechatOpenOptions.LinkRedirectURI,
	}

	signIn := signin.New(signin.Dependencies{
		SessionEstablisher: tokenCapabilities.SessionEstablisher,
		Authenticator:      authenticator,
		MethodRegistry:     method.DefaultSelector(),
		CredentialRecorder: credentialApp.NewRecorder(credentialApp.Dependencies{
			Credentials: infra.credentialRepo,
			LockoutPolicy: credentialDomain.LockoutPolicy{
				Enabled:      authOptions.PasswordLockout.Enabled,
				Threshold:    authOptions.PasswordLockout.Threshold,
				LockDuration: authOptions.PasswordLockout.LockDuration,
			},
		}),
		AdmissionPolicy: infra.admissionPolicy,
		ProofFactory:    proof.DefaultFactory(infra.externalResolver, challengeService),
	})

	userSessionService, err := session.NewApplicationService(session.Dependencies{
		Refresher: tokenCapabilities.Refresher,
		Revoker:   tokenCapabilities.Revoker,
		SignIn:    signIn,
	})
	if err != nil {
		return err
	}
	m.sessionService = userSessionService

	m.tokenCapabilities = tokenCapabilities
	m.sessionRevokeApp = session.NewRevoker(domain.sessionRevoker)

	logger := log.New(log.NewOptions())
	m.keyManagementApp = jwksApp.NewKeyManagementAppService(keyset.NewApplicationKeyReader(infra.keyManager), logger)
	keyPublisher := keyset.NewApplicationKeyPublisher(infra.keySetBuilder)
	m.keyPublishApp = jwksApp.NewKeyPublishAppService(keyPublisher, logger)
	m.keyLifecycleApp = jwksApp.NewKeyLifecycleAppService(
		keyset.NewApplicationKeyLifecycle(infra.keyRotation),
		keyPublisher,
		keyset.NewApplicationLifecycleObserver(),
		logger,
	)
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
		if infra.eventPublisher == nil {
			return nil, fmt.Errorf("sms.provider=mq requires the catalog event publisher")
		}
		return smsInfra.NewMQLoginOTPSenderWithPublisher(infra.eventPublisher), nil
	case "aliyun":
		validMinutes := int(smsOptions.LoginOTPTTL.Minutes())
		if validMinutes <= 0 {
			validMinutes = 5
		}
		return smsInfra.NewAliyunSender(smsInfra.AliyunConfig{
			AccessKeyID:     smsOptions.Aliyun.AccessKeyID,
			AccessKeySecret: smsOptions.Aliyun.AccessKeySecret,
			SignName:        smsOptions.Aliyun.SignName,
			TemplateCode:    smsOptions.Aliyun.TemplateCode,
			Endpoint:        smsOptions.Aliyun.Endpoint,
			CodeParamName:   smsOptions.Aliyun.CodeParamName,
			MinParamName:    smsOptions.Aliyun.MinParamName,
			ValidMinutes:    validMinutes,
			TimeoutMillis:   smsOptions.Aliyun.TimeoutMillis,
		})
	default:
		return nil, fmt.Errorf("unknown sms.provider=%q, expected one of log, mq, aliyun", smsProvider)
	}
}
