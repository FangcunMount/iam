package notificationrecipient

import (
	"context"
	"errors"
	"testing"

	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/loginidentity"
	"github.com/FangcunMount/iam/v5/internal/apiserver/domain/identity/useraccess"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type identityReaderStub struct {
	items  []*loginidentity.LoginIdentity
	err    error
	called bool
}

func (r *identityReaderStub) ListActiveMiniProgramByUserIDs(_ context.Context, _ []meta.ID, _ string) ([]*loginidentity.LoginIdentity, error) {
	r.called = true
	return r.items, r.err
}

type userStatusStub struct {
	byID map[meta.ID]useraccess.Status
	err  error
}

func (r userStatusStub) ReadUserStatus(_ context.Context, id meta.ID) (useraccess.Status, error) {
	return r.byID[id], r.err
}

func TestResolveExcludesInactiveUsersAndAmbiguousLegacyIdentifiers(t *testing.T) {
	const appID = "wx-app-a"
	reader := &identityReaderStub{items: []*loginidentity.LoginIdentity{
		{ID: 11, UserID: 1, Provider: loginidentity.ProviderWechatMinip, Realm: appID, Identifier: "openid-a", Status: loginidentity.StatusActive},
		{ID: 12, UserID: 2, Provider: loginidentity.ProviderWechatMinip, Realm: appID, Identifier: "openid-blocked", Status: loginidentity.StatusActive},
		{ID: 13, UserID: 1, Provider: loginidentity.ProviderWechatMinip, Realm: appID, Identifier: "possibly-unionid", Status: loginidentity.StatusActive, Meta: map[string]string{loginidentity.MetaLegacyIdentifierSemantics: loginidentity.LegacyIdentifierOpenOrUnion}},
		{ID: 14, UserID: 1, Provider: loginidentity.ProviderWechatMinip, Realm: "wx-other", Identifier: "wrong-app", Status: loginidentity.StatusActive},
	}}
	query := NewQuery(reader, userStatusStub{byID: map[meta.ID]useraccess.Status{1: useraccess.StatusActive, 2: useraccess.StatusBlocked}})
	items, err := query.Resolve(context.Background(), []meta.ID{1, 2}, appID)
	require.NoError(t, err)
	require.Equal(t, []Recipient{{UserID: 1, LoginIdentityID: 11, AppID: appID, OpenID: "openid-a"}}, items)
}

func TestResolveKeepsIdentityLookupFailuresDistinctFromNoRecipients(t *testing.T) {
	reader := &identityReaderStub{err: errors.New("database unavailable")}
	query := NewQuery(reader, userStatusStub{byID: map[meta.ID]useraccess.Status{1: useraccess.StatusActive}})
	items, err := query.Resolve(context.Background(), []meta.ID{1}, "wx-app-a")
	require.Error(t, err)
	require.Nil(t, items)
	require.True(t, reader.called)

	reader.err = nil
	items, err = query.Resolve(context.Background(), []meta.ID{1}, "wx-app-a")
	require.NoError(t, err)
	require.Empty(t, items)
}
