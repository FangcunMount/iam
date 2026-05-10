package linking

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	challengeapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/challenge"
	loginidentity "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestLinkPhoneRequiresChallengeAndCreatesIdentity(t *testing.T) {
	t.Parallel()

	repo := newLinkingIdentityRepoStub()
	challenge := &linkingChallengeStub{ok: true}
	service := NewService(Dependencies{
		LoginIdentities: repo,
		Challenge:       challenge,
		Now:             func() time.Time { return time.Unix(100, 0) },
	})

	result, err := service.LinkPhone(context.Background(), LinkPhoneCommand{
		UserID:  meta.FromUint64(100),
		Phone:   "13800138000",
		OTPCode: "123456",
	})

	require.NoError(t, err)
	require.False(t, result.Reused)
	require.Equal(t, loginidentity.ProviderPhone, result.Identity.Provider)
	require.Equal(t, "+8613800138000", result.Identity.Identifier)
	require.Equal(t, challengeapp.SceneLinkPhoneOTP, challenge.scene)
	require.Equal(t, "+8613800138000", challenge.phone)
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
	service := NewService(Dependencies{
		LoginIdentities: repo,
		Challenge:       &linkingChallengeStub{ok: true},
	})

	_, err := service.LinkPhone(context.Background(), LinkPhoneCommand{
		UserID:  meta.FromUint64(100),
		Phone:   phone,
		OTPCode: "123456",
	})

	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrLoginIdentityExists))
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
	service := NewService(Dependencies{LoginIdentities: repo})

	err := service.Unlink(context.Background(), UnlinkCommand{
		UserID:          meta.FromUint64(100),
		LoginIdentityID: meta.FromUint64(1),
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
	service := NewService(Dependencies{LoginIdentities: repo})

	err := service.Unlink(context.Background(), UnlinkCommand{
		UserID:          meta.FromUint64(100),
		LoginIdentityID: meta.FromUint64(1),
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

func (s *linkingIdentityRepoStub) store(identity *loginidentity.LoginIdentity) {
	s.byID[identity.ID] = identity
	s.byKey[linkingProviderKey(identity.Provider, identity.Realm, identity.Identifier)] = identity
}

func linkingProviderKey(provider loginidentity.Provider, realm, identifier string) string {
	return string(provider) + "|" + realm + "|" + identifier
}

type linkingChallengeStub struct {
	ok    bool
	scene string
	phone string
}

func (s *linkingChallengeStub) SendSMSOTP(_ context.Context, scene, phone string) error {
	s.scene = scene
	s.phone = phone
	return nil
}

func (s *linkingChallengeStub) CreateSMSOTP(context.Context, string, string, ...challengeapp.SMSOTPOption) (*challengeapp.SMSOTP, error) {
	return &challengeapp.SMSOTP{Code: "123456"}, nil
}

func (s *linkingChallengeStub) VerifyAndConsumeSMSOTP(_ context.Context, scene, phone, _ string) (bool, error) {
	s.scene = scene
	s.phone = phone
	return s.ok, nil
}

func (s *linkingChallengeStub) VerifyAndConsume(context.Context, string, string, string) bool {
	return s.ok
}

func (s *linkingChallengeStub) DeleteSMSOTP(context.Context, string, string) error {
	return nil
}
