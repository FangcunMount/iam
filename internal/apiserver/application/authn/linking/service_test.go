package linking

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	idpresolver "github.com/FangcunMount/iam/v3/internal/apiserver/application/idp/externalidentity"
	loginidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	idpidentity "github.com/FangcunMount/iam/v3/internal/apiserver/domain/idp/externalidentity"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestLinkPhoneRequiresChallengeAndCreatesIdentity(t *testing.T) {
	t.Parallel()

	repo := newLinkingIdentityRepoStub()
	challenge := &linkingChallengeStub{ok: true}
	linker := NewLinker(Dependencies{
		LoginIdentities: repo,
		PhoneLinkOTP:    challenge,
		Now:             func() time.Time { return time.Unix(100, 0) },
	})

	result, err := linker.Link(context.Background(), LinkRequest{
		UserID: meta.FromUint64(100),
		Input: LinkPhoneInput{
			Phone:   "13800138000",
			OTPCode: "123456",
		},
	})

	require.NoError(t, err)
	require.False(t, result.Reused)
	require.Equal(t, loginidentity.ProviderPhone, result.Identity.Provider)
	require.Equal(t, "+8613800138000", result.Identity.Identifier)
	require.Equal(t, "+8613800138000", challenge.phone)
	require.Equal(t, "123456", challenge.code)
}

func TestLinkPhoneRejectsProviderKeyOwnedByAnotherUser(t *testing.T) {
	t.Parallel()

	phone := "+8613800138000"
	repo := newLinkingIdentityRepoStub()
	repo.byKey[linkingProviderKey(loginidentity.ProviderPhone, loginidentity.RealmGlobal, phone)] = &loginidentity.LoginIdentity{
		ID:         meta.FromUint64(1),
		UserID:     meta.FromUint64(200),
		Provider:   loginidentity.ProviderPhone,
		Realm:      loginidentity.RealmGlobal,
		Identifier: phone,
		Status:     loginidentity.StatusActive,
	}
	linker := NewLinker(Dependencies{
		LoginIdentities: repo,
		PhoneLinkOTP:    &linkingChallengeStub{ok: true},
	})

	_, err := linker.Link(context.Background(), LinkRequest{
		UserID: meta.FromUint64(100),
		Input: LinkPhoneInput{
			Phone:   phone,
			OTPCode: "123456",
		},
	})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrLoginIdentityExists))
}

func TestLinkExternalIdentityUsesSingleResolvedProof(t *testing.T) {
	verifiedAt := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	wechatMiniKey, err := loginidentity.NewWechatMinipProviderKey("mini-app", "open-1", "union-1")
	require.NoError(t, err)
	wechatOpenKey, err := loginidentity.NewWechatOpenProviderKey("open-app", "open-2", "union-2")
	require.NoError(t, err)
	wecomKey, err := loginidentity.NewWecomProviderKey("corp-1", "user-1")
	require.NoError(t, err)
	tests := []struct {
		name        string
		provider    idpidentity.Provider
		realm       string
		identifiers map[idpidentity.IdentifierKind]string
		input       LinkLoginIdentityInput
		want        loginidentity.ProviderKey
	}{
		{
			name:     "wechat mini",
			provider: idpidentity.ProviderWechatMinip,
			realm:    "mini-app",
			identifiers: map[idpidentity.IdentifierKind]string{
				idpidentity.IdentifierOpenID:  "open-1",
				idpidentity.IdentifierUnionID: "union-1",
			},
			input: LinkWechatMiniInput{AppID: "mini-app", Code: "code-1"},
			want:  wechatMiniKey,
		},
		{
			name:     "wechat open",
			provider: idpidentity.ProviderWechatOpen,
			realm:    "open-app",
			identifiers: map[idpidentity.IdentifierKind]string{
				idpidentity.IdentifierOpenID:  "open-2",
				idpidentity.IdentifierUnionID: "union-2",
			},
			input: LinkWechatOpenInput{AppID: "open-app", Code: "code-2"},
			want:  wechatOpenKey,
		},
		{
			name:     "wecom",
			provider: idpidentity.ProviderWecom,
			realm:    "corp-1",
			identifiers: map[idpidentity.IdentifierKind]string{
				idpidentity.IdentifierUserID:     "user-1",
				idpidentity.IdentifierOpenUserID: "open-user-1",
			},
			input: LinkWecomInput{CorpID: "corp-1", Code: "code-3"},
			want:  wecomKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity := externalIdentityForTest(t, tt.provider, tt.realm, tt.identifiers, verifiedAt)
			resolver := &linkResolverStub{identity: identity}
			linker := NewLinker(Dependencies{
				LoginIdentities:  repoForExternalLink(),
				ExternalIdentity: resolver,
			})

			result, err := linker.Link(context.Background(), LinkRequest{
				UserID: meta.FromUint64(100),
				Input:  tt.input,
			})

			require.NoError(t, err)
			require.Equal(t, 1, resolver.calls)
			require.Equal(t, tt.provider, resolver.request.Provider)
			require.Equal(t, tt.want.Provider(), result.Identity.Provider)
			require.Equal(t, tt.want.Realm(), result.Identity.Realm)
			require.Equal(t, tt.want.Identifier(), result.Identity.Identifier)
			require.Equal(t, tt.want.GlobalIdentifier(), result.Identity.GlobalIdentifier)
			require.NotNil(t, result.Identity.VerifiedAt)
			require.Equal(t, verifiedAt, *result.Identity.VerifiedAt)
		})
	}
}

func TestUnlinkRejectsLastActiveLoginIdentity(t *testing.T) {
	t.Parallel()

	repo := newLinkingIdentityRepoStub()
	identity := &loginidentity.LoginIdentity{
		ID:         meta.FromUint64(1),
		UserID:     meta.FromUint64(100),
		Provider:   loginidentity.ProviderPhone,
		Realm:      loginidentity.RealmGlobal,
		Identifier: "+8613800138000",
		Status:     loginidentity.StatusActive,
	}
	repo.store(identity)
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	authenticatedAt := now.Add(-time.Minute)
	linker := NewLinker(Dependencies{
		LoginIdentities: repo,
		Now:             func() time.Time { return now },
	})

	err := linker.Unlink(context.Background(), UnlinkCommand{
		UserID:          meta.FromUint64(100),
		LoginIdentityID: meta.FromUint64(1),
		AuthenticatedAt: &authenticatedAt,
	})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrInvalidArgument))
	require.Equal(t, loginidentity.StatusActive, repo.byID[meta.FromUint64(1)].Status)
}

func TestUnlinkMarksIdentityDeletedWhenAnotherActiveIdentityRemains(t *testing.T) {
	t.Parallel()

	repo := newLinkingIdentityRepoStub()
	repo.store(&loginidentity.LoginIdentity{
		ID:         meta.FromUint64(1),
		UserID:     meta.FromUint64(100),
		Provider:   loginidentity.ProviderPhone,
		Realm:      loginidentity.RealmGlobal,
		Identifier: "+8613800138000",
		Status:     loginidentity.StatusActive,
	})
	repo.store(&loginidentity.LoginIdentity{
		ID:         meta.FromUint64(2),
		UserID:     meta.FromUint64(100),
		Provider:   loginidentity.ProviderUsername,
		Realm:      loginidentity.RealmDefault,
		Identifier: "zhangsan",
		Status:     loginidentity.StatusActive,
	})
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	authenticatedAt := now.Add(-time.Minute)
	linker := NewLinker(Dependencies{
		LoginIdentities:  repo,
		Now:              func() time.Time { return now },
		RecentAuthWindow: 10 * time.Minute,
	})

	err := linker.Unlink(context.Background(), UnlinkCommand{
		UserID:          meta.FromUint64(100),
		LoginIdentityID: meta.FromUint64(1),
		AuthenticatedAt: &authenticatedAt,
	})

	require.NoError(t, err)
	require.Equal(t, loginidentity.StatusDeleted, repo.byID[meta.FromUint64(1)].Status)
}

func TestUnlinkSensitiveIdentityRequiresRecentAuthentication(t *testing.T) {
	t.Parallel()

	repo := newLinkingIdentityRepoStub()
	repo.store(&loginidentity.LoginIdentity{
		ID:         meta.FromUint64(1),
		UserID:     meta.FromUint64(100),
		Provider:   loginidentity.ProviderUsername,
		Realm:      loginidentity.RealmDefault,
		Identifier: "zhangsan",
		Status:     loginidentity.StatusActive,
	})
	repo.store(&loginidentity.LoginIdentity{
		ID:         meta.FromUint64(2),
		UserID:     meta.FromUint64(100),
		Provider:   loginidentity.ProviderWechatMinip,
		Realm:      "wx-app",
		Identifier: "openid",
		Status:     loginidentity.StatusActive,
	})
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	linker := NewLinker(Dependencies{
		LoginIdentities:  repo,
		Now:              func() time.Time { return now },
		RecentAuthWindow: 10 * time.Minute,
	})

	err := linker.Unlink(context.Background(), UnlinkCommand{
		UserID:          meta.FromUint64(100),
		LoginIdentityID: meta.FromUint64(1),
	})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrReauthenticationRequired))
	require.Equal(t, loginidentity.StatusActive, repo.byID[meta.FromUint64(1)].Status)
}

func TestUnlinkCurrentSessionIdentityRequiresRecentAuthentication(t *testing.T) {
	t.Parallel()

	repo := newLinkingIdentityRepoStub()
	repo.store(&loginidentity.LoginIdentity{
		ID:         meta.FromUint64(1),
		UserID:     meta.FromUint64(100),
		Provider:   loginidentity.ProviderWechatMinip,
		Realm:      "wx-app",
		Identifier: "openid",
		Status:     loginidentity.StatusActive,
	})
	repo.store(&loginidentity.LoginIdentity{
		ID:         meta.FromUint64(2),
		UserID:     meta.FromUint64(100),
		Provider:   loginidentity.ProviderWecom,
		Realm:      "corp",
		Identifier: "userid",
		Status:     loginidentity.StatusActive,
	})
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	linker := NewLinker(Dependencies{
		LoginIdentities:  repo,
		Now:              func() time.Time { return now },
		RecentAuthWindow: 10 * time.Minute,
	})

	err := linker.Unlink(context.Background(), UnlinkCommand{
		UserID:                 meta.FromUint64(100),
		LoginIdentityID:        meta.FromUint64(1),
		CurrentLoginIdentityID: meta.FromUint64(1),
	})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrReauthenticationRequired))

	oldAuthTime := now.Add(-time.Hour)
	err = linker.Unlink(context.Background(), UnlinkCommand{
		UserID:                 meta.FromUint64(100),
		LoginIdentityID:        meta.FromUint64(1),
		CurrentLoginIdentityID: meta.FromUint64(1),
		AuthenticatedAt:        &oldAuthTime,
	})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrReauthenticationRequired))

	recentAuthTime := now.Add(-time.Minute)
	err = linker.Unlink(context.Background(), UnlinkCommand{
		UserID:                 meta.FromUint64(100),
		LoginIdentityID:        meta.FromUint64(1),
		CurrentLoginIdentityID: meta.FromUint64(1),
		AuthenticatedAt:        &recentAuthTime,
	})

	require.NoError(t, err)
	require.Equal(t, loginidentity.StatusDeleted, repo.byID[meta.FromUint64(1)].Status)
}

type linkingIdentityRepoStub struct {
	nextID meta.ID
	byID   map[meta.ID]*loginidentity.LoginIdentity
	byKey  map[string]*loginidentity.LoginIdentity
}

func newLinkingIdentityRepoStub() *linkingIdentityRepoStub {
	return &linkingIdentityRepoStub{
		nextID: meta.FromUint64(1000),
		byID:   map[meta.ID]*loginidentity.LoginIdentity{},
		byKey:  map[string]*loginidentity.LoginIdentity{},
	}
}

func (s *linkingIdentityRepoStub) Create(_ context.Context, identity *loginidentity.LoginIdentity) error {
	if identity.ID.IsZero() {
		s.nextID++
		identity.ID = s.nextID
	}
	s.store(identity)
	return nil
}

func (s *linkingIdentityRepoStub) GetByID(_ context.Context, id meta.ID) (*loginidentity.LoginIdentity, error) {
	return s.byID[id], nil
}

func (s *linkingIdentityRepoStub) GetByProviderKey(_ context.Context, provider loginidentity.Provider, realm, identifier string) (*loginidentity.LoginIdentity, error) {
	return s.byKey[linkingProviderKey(provider, realm, identifier)], nil
}

func (s *linkingIdentityRepoStub) GetByGlobalIdentifier(context.Context, loginidentity.Provider, string) (*loginidentity.LoginIdentity, error) {
	return nil, nil
}

func (s *linkingIdentityRepoStub) ListByUserID(_ context.Context, userID meta.ID) ([]*loginidentity.LoginIdentity, error) {
	var out []*loginidentity.LoginIdentity
	for _, identity := range s.byID {
		if identity.UserID == userID {
			out = append(out, identity)
		}
	}
	return out, nil
}

func (s *linkingIdentityRepoStub) UpdateStatus(_ context.Context, id meta.ID, status loginidentity.Status) error {
	if identity := s.byID[id]; identity != nil {
		identity.Status = status
	}
	return nil
}

func (s *linkingIdentityRepoStub) UnlinkOwnedUnlessLastActive(
	_ context.Context,
	userID meta.ID,
	loginIdentityID meta.ID,
) (UnlinkOutcome, error) {
	identity := s.byID[loginIdentityID]
	if identity == nil || identity.UserID != userID {
		return UnlinkOutcomeNotFound, nil
	}
	activeCount := 0
	for _, candidate := range s.byID {
		if candidate.UserID == userID && candidate.Status == loginidentity.StatusActive {
			activeCount++
		}
	}
	if identity.Status == loginidentity.StatusActive && activeCount <= 1 {
		return UnlinkOutcomeLastActive, nil
	}
	identity.Status = loginidentity.StatusDeleted
	return UnlinkOutcomeUnlinked, nil
}

func (s *linkingIdentityRepoStub) store(identity *loginidentity.LoginIdentity) {
	s.byID[identity.ID] = identity
	s.byKey[linkingProviderKey(identity.Provider, identity.Realm, identity.Identifier)] = identity
}

func linkingProviderKey(provider loginidentity.Provider, realm, identifier string) string {
	return string(provider) + "|" + realm + "|" + identifier
}

type linkingChallengeStub struct {
	ok    bool
	phone string
	code  string
}

type linkResolverStub struct {
	identity idpidentity.ExternalIdentity
	request  idpresolver.ResolveRequest
	calls    int
}

func (s *linkResolverStub) Resolve(_ context.Context, request idpresolver.ResolveRequest) (idpidentity.ExternalIdentity, error) {
	s.calls++
	s.request = request
	return s.identity, nil
}

func externalIdentityForTest(
	t *testing.T,
	provider idpidentity.Provider,
	realm string,
	values map[idpidentity.IdentifierKind]string,
	verifiedAt time.Time,
) idpidentity.ExternalIdentity {
	t.Helper()
	identifiers := make([]idpidentity.Identifier, 0, len(values))
	for kind, value := range values {
		identifier, err := idpidentity.NewIdentifier(kind, value)
		require.NoError(t, err)
		identifiers = append(identifiers, identifier)
	}
	identity, err := idpidentity.New(provider, realm, identifiers, verifiedAt)
	require.NoError(t, err)
	return identity
}

func repoForExternalLink() *linkingIdentityRepoStub {
	return newLinkingIdentityRepoStub()
}

func (s *linkingChallengeStub) VerifyAndConsumePhoneLinkOTP(_ context.Context, phone, code string) bool {
	s.phone = phone
	s.code = code
	return s.ok
}
