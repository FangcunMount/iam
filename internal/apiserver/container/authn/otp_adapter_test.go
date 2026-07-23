package authn

import (
	"bytes"
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	linkingApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/linking"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/method"
	"github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/signin/proof"
	tokenApp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	sessionDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPhoneOTPLoginConsumesChallengeThroughExplicitAdapter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	challengeRepo := newAuthnChallengeRepoStub()
	smsSender := &authnSMSSenderStub{}
	challengeService := newAuthnChallengeService(challengeRepo, smsSender)

	phone := "+8613800138000"
	require.NoError(t, challengeService.SendLoginPhoneOTP(ctx, phone))
	require.NotEmpty(t, smsSender.code)

	userID := meta.FromUint64(1001)
	loginIdentityID := meta.FromUint64(2001)
	identityRepo := &authnLoginIdentityRepoStub{
		lookup: &authentication.LoginIdentityLookup{
			LoginIdentityID: loginIdentityID,
			UserID:          userID,
			Provider:        loginidentity.ProviderPhone,
			Realm:           loginidentity.RealmGlobal,
			Identifier:      phone,
			Status:          loginidentity.StatusActive,
		},
	}
	authenticator := authentication.NewAuthenticator(
		newPhoneOTPAuthStrategy(identityRepo, challengeService),
	)
	tokenService := &authnTokenServiceStub{}
	signIn := signin.New(signin.Dependencies{
		TokenService:   tokenService,
		Authenticator:  authenticator,
		MethodRegistry: method.DefaultSelector(),
		ProofFactory:   proof.DefaultFactory(nil, nil, proof.WecomConfig{}, nil),
		AccessChecker:  authnSubjectAccessEvaluatorStub{},
	})

	result, err := signIn.Execute(ctx, method.LoginRequest{
		AuthMethod: method.AuthMethodPhoneOTP,
		Payload: method.PhoneOTPPayload{
			PhoneE164: phone,
			OTP:       smsSender.code,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, userID, result.UserID)
	require.Equal(t, loginIdentityID, result.LoginIdentityID)
	require.Empty(t, challengeRepo.items, "successful login must consume the OTP challenge")

	result, err = signIn.Execute(ctx, method.LoginRequest{
		AuthMethod: method.AuthMethodPhoneOTP,
		Payload: method.PhoneOTPPayload{
			PhoneE164: phone,
			OTP:       smsSender.code,
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, code.ErrOTPInvalid, perrors.ParseCoder(err).Code())
}

func TestPhoneLinkConsumesChallengeThroughExplicitAdapter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	challengeRepo := newAuthnChallengeRepoStub()
	smsSender := &authnSMSSenderStub{}
	challengeService := newAuthnChallengeService(challengeRepo, smsSender)

	phone := "+8613800138000"
	require.NoError(t, challengeService.SendPhoneLinkOTP(ctx, phone))
	require.NotEmpty(t, smsSender.code)

	identityRepo := newAuthnLinkingIdentityRepoStub()
	linker := linkingApp.NewLinker(linkingApp.Dependencies{
		LoginIdentities: identityRepo,
		PhoneLinkOTP:    newPhoneLinkOTPVerifierAdapter(challengeService),
		Now:             func() time.Time { return time.Unix(100, 0) },
	})

	result, err := linker.Link(ctx, linkingApp.LinkRequest{
		UserID: meta.FromUint64(1001),
		Input: linkingApp.LinkPhoneInput{
			Phone:   phone,
			OTPCode: smsSender.code,
		},
	})

	require.NoError(t, err)
	require.False(t, result.Reused)
	require.Equal(t, loginidentity.ProviderPhone, result.Identity.Provider)
	require.Equal(t, phone, result.Identity.Identifier)
	require.Empty(t, challengeRepo.items, "successful phone linking must consume the OTP challenge")

	result, err = linker.Link(ctx, linkingApp.LinkRequest{
		UserID: meta.FromUint64(1001),
		Input: linkingApp.LinkPhoneInput{
			Phone:   phone,
			OTPCode: smsSender.code,
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, code.ErrInvalidCredential, perrors.ParseCoder(err).Code())
}

func newAuthnChallengeService(repo *authnChallengeRepoStub, smsSender *authnSMSSenderStub) challengeApp.Service {
	return challengeApp.NewService(
		repo,
		challengeApp.SMSOTPDelivery{
			Gate:     authnSMSOTPGateStub{},
			SMS:      smsSender,
			TTL:      time.Minute,
			Cooldown: time.Minute,
			CodeLen:  6,
		},
		challengeApp.NewCreator(repo),
		challengeApp.NewVerifier(repo),
	)
}

type authnChallengeRepoStub struct {
	items map[string]*challengeDomain.AuthChallenge
}

func newAuthnChallengeRepoStub() *authnChallengeRepoStub {
	return &authnChallengeRepoStub{items: map[string]*challengeDomain.AuthChallenge{}}
}

func (s *authnChallengeRepoStub) Create(_ context.Context, challenge *challengeDomain.AuthChallenge) error {
	s.items[challenge.ID] = challenge
	return nil
}

func (s *authnChallengeRepoStub) Get(_ context.Context, id string) (*challengeDomain.AuthChallenge, error) {
	return s.items[id], nil
}

func (s *authnChallengeRepoStub) ConsumeIfSecretMatches(_ context.Context, id string, expectedHash []byte) (bool, error) {
	item := s.items[id]
	if item == nil || !bytes.Equal(item.SecretHash, expectedHash) {
		return false, nil
	}
	delete(s.items, id)
	return true, nil
}

func (s *authnChallengeRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}

type authnSMSOTPGateStub struct{}

func (authnSMSOTPGateStub) TryAcquire(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}

type authnSMSSenderStub struct {
	phone string
	code  string
}

func (s *authnSMSSenderStub) SendLoginOTP(_ context.Context, phoneE164, code string) error {
	s.phone = phoneE164
	s.code = code
	return nil
}

type authnLoginIdentityRepoStub struct {
	lookup *authentication.LoginIdentityLookup
}

func (s *authnLoginIdentityRepoStub) FindUsernameIdentity(context.Context, meta.ID, string) (*authentication.LoginIdentityLookup, error) {
	return nil, nil
}

func (s *authnLoginIdentityRepoStub) FindLoginIdentityByProviderKey(_ context.Context, provider loginidentity.Provider, realm, identifier string) (*authentication.LoginIdentityLookup, error) {
	if s.lookup == nil {
		return nil, nil
	}
	if s.lookup.Provider == provider && s.lookup.Realm == realm && s.lookup.Identifier == identifier {
		return s.lookup, nil
	}
	return nil, nil
}

func (s *authnLoginIdentityRepoStub) FindLoginIdentityByGlobalIdentifier(context.Context, loginidentity.Provider, string) (*authentication.LoginIdentityLookup, error) {
	return nil, nil
}

func (s *authnLoginIdentityRepoStub) IsLoginIdentityActive(_ context.Context, loginIdentityID meta.ID) (bool, error) {
	return s.lookup != nil && s.lookup.LoginIdentityID == loginIdentityID && s.lookup.Status == loginidentity.StatusActive, nil
}

type authnLinkingIdentityRepoStub struct {
	nextID meta.ID
	byID   map[meta.ID]*loginidentity.LoginIdentity
	byKey  map[string]*loginidentity.LoginIdentity
}

func newAuthnLinkingIdentityRepoStub() *authnLinkingIdentityRepoStub {
	return &authnLinkingIdentityRepoStub{
		nextID: meta.FromUint64(3000),
		byID:   map[meta.ID]*loginidentity.LoginIdentity{},
		byKey:  map[string]*loginidentity.LoginIdentity{},
	}
}

func (s *authnLinkingIdentityRepoStub) Create(_ context.Context, identity *loginidentity.LoginIdentity) error {
	if identity.ID.IsZero() {
		s.nextID++
		identity.ID = s.nextID
	}
	s.store(identity)
	return nil
}

func (s *authnLinkingIdentityRepoStub) GetByID(_ context.Context, id meta.ID) (*loginidentity.LoginIdentity, error) {
	return s.byID[id], nil
}

func (s *authnLinkingIdentityRepoStub) GetByProviderKey(_ context.Context, provider loginidentity.Provider, realm, identifier string) (*loginidentity.LoginIdentity, error) {
	return s.byKey[authnLinkingProviderKey(provider, realm, identifier)], nil
}

func (s *authnLinkingIdentityRepoStub) GetByGlobalIdentifier(context.Context, loginidentity.Provider, string) (*loginidentity.LoginIdentity, error) {
	return nil, nil
}

func (s *authnLinkingIdentityRepoStub) ListByUserID(_ context.Context, userID meta.ID) ([]*loginidentity.LoginIdentity, error) {
	out := make([]*loginidentity.LoginIdentity, 0)
	for _, identity := range s.byID {
		if identity.UserID == userID {
			out = append(out, identity)
		}
	}
	return out, nil
}

func (s *authnLinkingIdentityRepoStub) UpdateStatus(_ context.Context, id meta.ID, status loginidentity.Status) error {
	if identity := s.byID[id]; identity != nil {
		identity.Status = status
	}
	return nil
}

func (s *authnLinkingIdentityRepoStub) store(identity *loginidentity.LoginIdentity) {
	s.byID[identity.ID] = identity
	s.byKey[authnLinkingProviderKey(identity.Provider, identity.Realm, identity.Identifier)] = identity
}

func authnLinkingProviderKey(provider loginidentity.Provider, realm, identifier string) string {
	return string(provider) + "|" + realm + "|" + identifier
}

type authnTokenServiceStub struct{}

type authnSubjectAccessEvaluatorStub struct{}

func (authnSubjectAccessEvaluatorStub) Evaluate(context.Context, meta.ID, meta.ID) (sessionDomain.SubjectAccessDecision, error) {
	return sessionDomain.SubjectAccessDecision{Status: sessionDomain.SubjectAccessActive}, nil
}

func (s *authnTokenServiceStub) IssueToken(_ context.Context, principal *authentication.Principal) (*tokenApp.TokenPair, error) {
	access := tokenApp.NewAccessToken(
		"access-id",
		"access-token",
		principal.SessionID,
		principal.UserID,
		principal.LoginIdentityID,
		principal.TenantID,
		time.Minute,
	)
	refresh := tokenApp.NewRefreshToken(
		"refresh-id",
		"refresh-token",
		principal.SessionID,
		principal.UserID,
		principal.LoginIdentityID,
		principal.TenantID,
		principal.AMR,
		nil,
		time.Hour,
	)
	return tokenApp.NewTokenPair(access, refresh), nil
}

func (s *authnTokenServiceStub) IssueServiceToken(context.Context, tokenApp.IssueServiceTokenRequest) (*tokenApp.TokenIssueResult, error) {
	return nil, nil
}

func (s *authnTokenServiceStub) RefreshToken(context.Context, string) (*tokenApp.TokenRefreshResult, error) {
	return nil, nil
}

func (s *authnTokenServiceStub) RevokeAccessToken(context.Context, string) error {
	return nil
}

func (s *authnTokenServiceStub) RevokeRefreshToken(context.Context, string) error {
	return nil
}

func (s *authnTokenServiceStub) VerifyToken(context.Context, tokenApp.VerifyTokenRequest) (*tokenApp.TokenVerifyResult, error) {
	return nil, nil
}
