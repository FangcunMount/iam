package assembler

import (
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
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPhoneOTPLoginConsumesChallengeThroughExplicitAdapter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	challengeRepo := newAssemblerChallengeRepoStub()
	smsSender := &assemblerSMSSenderStub{}
	challengeService := newAssemblerChallengeService(challengeRepo, smsSender)

	phone := "+8613800138000"
	require.NoError(t, challengeService.SendLoginPhoneOTP(ctx, phone))
	require.NotEmpty(t, smsSender.code)

	userID := meta.FromUint64(1001)
	loginIdentityID := meta.FromUint64(2001)
	identityRepo := &assemblerLoginIdentityRepoStub{
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
	tokenService := &assemblerTokenServiceStub{}
	signIn := signin.New(signin.Dependencies{
		TokenService:   tokenService,
		Authenticator:  authenticator,
		MethodRegistry: method.DefaultSelector(),
		ProofFactory:   proof.DefaultFactory(nil, nil, proof.WecomConfig{}, nil),
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
	challengeRepo := newAssemblerChallengeRepoStub()
	smsSender := &assemblerSMSSenderStub{}
	challengeService := newAssemblerChallengeService(challengeRepo, smsSender)

	phone := "+8613800138000"
	require.NoError(t, challengeService.SendPhoneLinkOTP(ctx, phone))
	require.NotEmpty(t, smsSender.code)

	identityRepo := newAssemblerLinkingIdentityRepoStub()
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

func newAssemblerChallengeService(repo *assemblerChallengeRepoStub, smsSender *assemblerSMSSenderStub) challengeApp.Service {
	return challengeApp.NewService(
		repo,
		challengeApp.SMSOTPDelivery{
			Gate:     assemblerSMSOTPGateStub{},
			SMS:      smsSender,
			TTL:      time.Minute,
			Cooldown: time.Minute,
			CodeLen:  6,
		},
		challengeApp.NewCreator(repo),
		challengeApp.NewVerifier(repo),
	)
}

type assemblerChallengeRepoStub struct {
	items map[string]*challengeDomain.AuthChallenge
}

func newAssemblerChallengeRepoStub() *assemblerChallengeRepoStub {
	return &assemblerChallengeRepoStub{items: map[string]*challengeDomain.AuthChallenge{}}
}

func (s *assemblerChallengeRepoStub) Create(_ context.Context, challenge *challengeDomain.AuthChallenge) error {
	s.items[challenge.ID] = challenge
	return nil
}

func (s *assemblerChallengeRepoStub) Get(_ context.Context, id string) (*challengeDomain.AuthChallenge, error) {
	return s.items[id], nil
}

func (s *assemblerChallengeRepoStub) Consume(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}

func (s *assemblerChallengeRepoStub) Delete(_ context.Context, id string) error {
	delete(s.items, id)
	return nil
}

type assemblerSMSOTPGateStub struct{}

func (assemblerSMSOTPGateStub) TryAcquire(context.Context, string, string, time.Duration) (bool, error) {
	return true, nil
}

type assemblerSMSSenderStub struct {
	phone string
	code  string
}

func (s *assemblerSMSSenderStub) SendLoginOTP(_ context.Context, phoneE164, code string) error {
	s.phone = phoneE164
	s.code = code
	return nil
}

type assemblerLoginIdentityRepoStub struct {
	lookup *authentication.LoginIdentityLookup
}

func (s *assemblerLoginIdentityRepoStub) FindUsernameIdentity(context.Context, meta.ID, string) (*authentication.LoginIdentityLookup, error) {
	return nil, nil
}

func (s *assemblerLoginIdentityRepoStub) FindLoginIdentityByProviderKey(_ context.Context, provider loginidentity.Provider, realm, identifier string) (*authentication.LoginIdentityLookup, error) {
	if s.lookup == nil {
		return nil, nil
	}
	if s.lookup.Provider == provider && s.lookup.Realm == realm && s.lookup.Identifier == identifier {
		return s.lookup, nil
	}
	return nil, nil
}

func (s *assemblerLoginIdentityRepoStub) FindLoginIdentityByGlobalIdentifier(context.Context, loginidentity.Provider, string) (*authentication.LoginIdentityLookup, error) {
	return nil, nil
}

func (s *assemblerLoginIdentityRepoStub) IsLoginIdentityActive(_ context.Context, loginIdentityID meta.ID) (bool, error) {
	return s.lookup != nil && s.lookup.LoginIdentityID == loginIdentityID && s.lookup.Status == loginidentity.StatusActive, nil
}

type assemblerLinkingIdentityRepoStub struct {
	nextID meta.ID
	byID   map[meta.ID]*loginidentity.LoginIdentity
	byKey  map[string]*loginidentity.LoginIdentity
}

func newAssemblerLinkingIdentityRepoStub() *assemblerLinkingIdentityRepoStub {
	return &assemblerLinkingIdentityRepoStub{
		nextID: meta.FromUint64(3000),
		byID:   map[meta.ID]*loginidentity.LoginIdentity{},
		byKey:  map[string]*loginidentity.LoginIdentity{},
	}
}

func (s *assemblerLinkingIdentityRepoStub) Create(_ context.Context, identity *loginidentity.LoginIdentity) error {
	if identity.ID.IsZero() {
		s.nextID++
		identity.ID = s.nextID
	}
	s.store(identity)
	return nil
}

func (s *assemblerLinkingIdentityRepoStub) GetByID(_ context.Context, id meta.ID) (*loginidentity.LoginIdentity, error) {
	return s.byID[id], nil
}

func (s *assemblerLinkingIdentityRepoStub) GetByProviderKey(_ context.Context, provider loginidentity.Provider, realm, identifier string) (*loginidentity.LoginIdentity, error) {
	return s.byKey[assemblerLinkingProviderKey(provider, realm, identifier)], nil
}

func (s *assemblerLinkingIdentityRepoStub) GetByGlobalIdentifier(context.Context, loginidentity.Provider, string) (*loginidentity.LoginIdentity, error) {
	return nil, nil
}

func (s *assemblerLinkingIdentityRepoStub) ListByUserID(_ context.Context, userID meta.ID) ([]*loginidentity.LoginIdentity, error) {
	out := make([]*loginidentity.LoginIdentity, 0)
	for _, identity := range s.byID {
		if identity.UserID == userID {
			out = append(out, identity)
		}
	}
	return out, nil
}

func (s *assemblerLinkingIdentityRepoStub) UpdateStatus(_ context.Context, id meta.ID, status loginidentity.Status) error {
	if identity := s.byID[id]; identity != nil {
		identity.Status = status
	}
	return nil
}

func (s *assemblerLinkingIdentityRepoStub) store(identity *loginidentity.LoginIdentity) {
	s.byID[identity.ID] = identity
	s.byKey[assemblerLinkingProviderKey(identity.Provider, identity.Realm, identity.Identifier)] = identity
}

func assemblerLinkingProviderKey(provider loginidentity.Provider, realm, identifier string) string {
	return string(provider) + "|" + realm + "|" + identifier
}

type assemblerTokenServiceStub struct{}

func (s *assemblerTokenServiceStub) IssueToken(_ context.Context, principal *authentication.Principal) (*tokenApp.TokenPair, error) {
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

func (s *assemblerTokenServiceStub) IssueServiceToken(context.Context, tokenApp.IssueServiceTokenRequest) (*tokenApp.TokenIssueResult, error) {
	return nil, nil
}

func (s *assemblerTokenServiceStub) RefreshToken(context.Context, string) (*tokenApp.TokenRefreshResult, error) {
	return nil, nil
}

func (s *assemblerTokenServiceStub) RevokeAccessToken(context.Context, string) error {
	return nil
}

func (s *assemblerTokenServiceStub) RevokeRefreshToken(context.Context, string) error {
	return nil
}

func (s *assemblerTokenServiceStub) VerifyToken(context.Context, tokenApp.VerifyTokenRequest) (*tokenApp.TokenVerifyResult, error) {
	return nil, nil
}
