package session

import (
	"context"
	"testing"

	loginidentitydomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	userdomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/user"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestSubjectAccessEvaluatorAcceptsActiveLoginIdentityAndUser(t *testing.T) {
	ctx := context.Background()
	userID := meta.FromUint64(1001)
	loginIdentityID := meta.FromUint64(2001)

	users := &subjectAccessUserRepoStub{
		byID: map[meta.ID]*userdomain.User{
			userID: {ID: userID, Name: "active", Status: userdomain.UserActive},
		},
	}
	identities := &subjectAccessLoginIdentityRepoStub{
		byID: map[meta.ID]*loginidentitydomain.LoginIdentity{
			loginIdentityID: {
				ID:         loginIdentityID,
				UserID:     userID,
				Provider:   loginidentitydomain.ProviderUsername,
				Realm:      loginidentitydomain.RealmDefault,
				Identifier: "zhangsan",
				Status:     loginidentitydomain.StatusActive,
			},
		},
	}
	evaluator := NewSubjectAccessEvaluator(users, identities)

	decision, err := evaluator.Evaluate(ctx, userID, loginIdentityID)
	require.NoError(t, err)
	require.Equal(t, SubjectAccessActive, decision.Status)
	require.Equal(t, loginIdentityID, decision.LoginIdentityID)
}

func TestSubjectAccessEvaluatorRejectsDisabledLoginIdentityAndBlockedUser(t *testing.T) {
	ctx := context.Background()
	userID := meta.FromUint64(1001)
	disabledIdentityID := meta.FromUint64(2001)
	blockedIdentityID := meta.FromUint64(2002)

	users := &subjectAccessUserRepoStub{
		byID: map[meta.ID]*userdomain.User{
			userID: {ID: userID, Name: "blocked", Status: userdomain.UserBlocked},
		},
	}
	identities := &subjectAccessLoginIdentityRepoStub{
		byID: map[meta.ID]*loginidentitydomain.LoginIdentity{
			disabledIdentityID: {
				ID:         disabledIdentityID,
				UserID:     userID,
				Provider:   loginidentitydomain.ProviderPhone,
				Realm:      loginidentitydomain.RealmGlobal,
				Identifier: "+8613811112222",
				Status:     loginidentitydomain.StatusDisabled,
			},
			blockedIdentityID: {
				ID:         blockedIdentityID,
				UserID:     userID,
				Provider:   loginidentitydomain.ProviderPhone,
				Realm:      loginidentitydomain.RealmGlobal,
				Identifier: "+8613811113333",
				Status:     loginidentitydomain.StatusActive,
			},
		},
	}
	evaluator := NewSubjectAccessEvaluator(users, identities)

	decision, err := evaluator.Evaluate(ctx, userID, disabledIdentityID)
	require.NoError(t, err)
	require.Equal(t, SubjectAccessDisabled, decision.Status)

	decision, err = evaluator.Evaluate(ctx, userID, blockedIdentityID)
	require.NoError(t, err)
	require.Equal(t, SubjectAccessBlocked, decision.Status)
}

func TestSubjectAccessEvaluatorDistinguishesInactiveUser(t *testing.T) {
	ctx := context.Background()
	userID := meta.FromUint64(1001)
	loginIdentityID := meta.FromUint64(2001)
	users := &subjectAccessUserRepoStub{
		byID: map[meta.ID]*userdomain.User{
			userID: {ID: userID, Name: "inactive", Status: userdomain.UserInactive},
		},
	}
	identities := &subjectAccessLoginIdentityRepoStub{
		byID: map[meta.ID]*loginidentitydomain.LoginIdentity{
			loginIdentityID: {
				ID:         loginIdentityID,
				UserID:     userID,
				Provider:   loginidentitydomain.ProviderUsername,
				Realm:      loginidentitydomain.RealmDefault,
				Identifier: "inactive-user",
				Status:     loginidentitydomain.StatusActive,
			},
		},
	}

	decision, err := NewSubjectAccessEvaluator(users, identities).Evaluate(ctx, userID, loginIdentityID)
	require.NoError(t, err)
	require.Equal(t, SubjectAccessInactive, decision.Status)
}

type subjectAccessUserRepoStub struct {
	byID map[meta.ID]*userdomain.User
}

func (s *subjectAccessUserRepoStub) Create(context.Context, *userdomain.User) error { return nil }
func (s *subjectAccessUserRepoStub) FindByID(_ context.Context, id meta.ID) (*userdomain.User, error) {
	return s.byID[id], nil
}
func (s *subjectAccessUserRepoStub) FindByIDs(_ context.Context, ids []meta.ID) (map[meta.ID]*userdomain.User, error) {
	out := make(map[meta.ID]*userdomain.User, len(ids))
	for _, id := range ids {
		if user := s.byID[id]; user != nil {
			out[id] = user
		}
	}
	return out, nil
}
func (s *subjectAccessUserRepoStub) FindByPhone(context.Context, meta.Phone) (*userdomain.User, error) {
	return nil, nil
}
func (s *subjectAccessUserRepoStub) Update(context.Context, *userdomain.User) error { return nil }

type subjectAccessLoginIdentityRepoStub struct {
	byID map[meta.ID]*loginidentitydomain.LoginIdentity
}

func (s *subjectAccessLoginIdentityRepoStub) Create(context.Context, *loginidentitydomain.LoginIdentity) error {
	return nil
}
func (s *subjectAccessLoginIdentityRepoStub) GetByID(_ context.Context, id meta.ID) (*loginidentitydomain.LoginIdentity, error) {
	return s.byID[id], nil
}
func (s *subjectAccessLoginIdentityRepoStub) GetByProviderKey(context.Context, loginidentitydomain.Provider, string, string) (*loginidentitydomain.LoginIdentity, error) {
	return nil, nil
}
func (s *subjectAccessLoginIdentityRepoStub) GetByGlobalIdentifier(context.Context, loginidentitydomain.Provider, string) (*loginidentitydomain.LoginIdentity, error) {
	return nil, nil
}
func (s *subjectAccessLoginIdentityRepoStub) ListByUserID(context.Context, meta.ID) ([]*loginidentitydomain.LoginIdentity, error) {
	return nil, nil
}
func (s *subjectAccessLoginIdentityRepoStub) UpdateStatus(context.Context, meta.ID, loginidentitydomain.Status) error {
	return nil
}
