package admission

import (
	"context"
	"testing"

	loginidentitydomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestPolicyAdmitsActiveLoginIdentityAndUser(t *testing.T) {
	ctx := context.Background()
	userID := meta.FromUint64(1001)
	loginIdentityID := meta.FromUint64(2001)

	users := &userStatusReaderStub{byID: map[meta.ID]useraccess.Status{userID: useraccess.StatusActive}}
	identities := &loginIdentityRepoStub{byID: map[meta.ID]*loginidentitydomain.LoginIdentity{
		loginIdentityID: {
			ID: loginIdentityID, UserID: userID,
			Provider: loginidentitydomain.ProviderUsername, Realm: loginidentitydomain.RealmDefault,
			Identifier: "zhangsan", Status: loginidentitydomain.StatusActive,
		},
	}}

	decision, err := NewPolicy(users, identities).Evaluate(ctx, userID, loginIdentityID)
	require.NoError(t, err)
	require.True(t, decision.IsAdmitted())
	require.Equal(t, StatusActive, decision.Status)
	require.Equal(t, loginIdentityID, decision.LoginIdentityID)
}

func TestPolicyDeniesDisabledLoginIdentityAndBlockedUser(t *testing.T) {
	ctx := context.Background()
	userID := meta.FromUint64(1001)
	disabledIdentityID := meta.FromUint64(2001)
	blockedIdentityID := meta.FromUint64(2002)

	users := &userStatusReaderStub{byID: map[meta.ID]useraccess.Status{userID: useraccess.StatusBlocked}}
	identities := &loginIdentityRepoStub{byID: map[meta.ID]*loginidentitydomain.LoginIdentity{
		disabledIdentityID: {
			ID: disabledIdentityID, UserID: userID,
			Provider: loginidentitydomain.ProviderPhone, Realm: loginidentitydomain.RealmGlobal,
			Identifier: "+8613811112222", Status: loginidentitydomain.StatusDisabled,
		},
		blockedIdentityID: {
			ID: blockedIdentityID, UserID: userID,
			Provider: loginidentitydomain.ProviderPhone, Realm: loginidentitydomain.RealmGlobal,
			Identifier: "+8613811113333", Status: loginidentitydomain.StatusActive,
		},
	}}

	decision, err := NewPolicy(users, identities).Evaluate(ctx, userID, disabledIdentityID)
	require.NoError(t, err)
	require.Equal(t, StatusDisabled, decision.Status)

	decision, err = NewPolicy(users, identities).Evaluate(ctx, userID, blockedIdentityID)
	require.NoError(t, err)
	require.Equal(t, StatusBlocked, decision.Status)
}

func TestPolicyDistinguishesInactiveUser(t *testing.T) {
	ctx := context.Background()
	userID := meta.FromUint64(1001)
	loginIdentityID := meta.FromUint64(2001)
	users := &userStatusReaderStub{byID: map[meta.ID]useraccess.Status{userID: useraccess.StatusInactive}}
	identities := &loginIdentityRepoStub{byID: map[meta.ID]*loginidentitydomain.LoginIdentity{
		loginIdentityID: {
			ID: loginIdentityID, UserID: userID,
			Provider: loginidentitydomain.ProviderUsername, Realm: loginidentitydomain.RealmDefault,
			Identifier: "inactive-user", Status: loginidentitydomain.StatusActive,
		},
	}}

	decision, err := NewPolicy(users, identities).Evaluate(ctx, userID, loginIdentityID)
	require.NoError(t, err)
	require.Equal(t, StatusInactive, decision.Status)
}

type userStatusReaderStub struct {
	byID map[meta.ID]useraccess.Status
}

func (s *userStatusReaderStub) ReadUserStatus(_ context.Context, id meta.ID) (useraccess.Status, error) {
	status, ok := s.byID[id]
	if !ok {
		return useraccess.StatusMissing, nil
	}
	return status, nil
}

type loginIdentityRepoStub struct {
	byID map[meta.ID]*loginidentitydomain.LoginIdentity
}

func (s *loginIdentityRepoStub) Create(context.Context, *loginidentitydomain.LoginIdentity) error {
	return nil
}
func (s *loginIdentityRepoStub) GetByID(_ context.Context, id meta.ID) (*loginidentitydomain.LoginIdentity, error) {
	return s.byID[id], nil
}
func (s *loginIdentityRepoStub) GetByProviderKey(context.Context, loginidentitydomain.Provider, string, string) (*loginidentitydomain.LoginIdentity, error) {
	return nil, nil
}
func (s *loginIdentityRepoStub) GetByGlobalIdentifier(context.Context, loginidentitydomain.Provider, string) (*loginidentitydomain.LoginIdentity, error) {
	return nil, nil
}
func (s *loginIdentityRepoStub) ListByUserID(context.Context, meta.ID) ([]*loginidentitydomain.LoginIdentity, error) {
	return nil, nil
}
func (s *loginIdentityRepoStub) UpdateStatus(context.Context, meta.ID, loginidentitydomain.Status) error {
	return nil
}
