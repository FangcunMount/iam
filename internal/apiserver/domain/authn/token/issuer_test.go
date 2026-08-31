package token

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/stretchr/testify/require"
)

func TestAuthenticationIssuerCreatesGrantAndPersistsInitialRefreshToken(t *testing.T) {
	t.Parallel()

	sess := testActiveSession()
	creator := sessionCreatorStub{session: sess}
	store := newAtomicTokenStoreStub()
	minter := &recordingTokenPairMinter{
		set: NewUserTokenSet(
			NewAccessToken("access-id", "access-value", sess.SessionID, sess.UserID, sess.LoginIdentityID, sess.TenantID, time.Minute),
			testRefreshToken("refresh-id", "refresh-value"),
		),
	}
	principal := &authentication.Principal{
		UserID:          sess.UserID,
		LoginIdentityID: sess.LoginIdentityID,
		TenantID:        sess.TenantID,
	}

	grant, err := newAuthenticationIssuer(creator, store, minter).Issue(context.Background(), principal)

	require.NoError(t, err)
	require.Same(t, sess, grant.Session)
	require.Same(t, minter.set, grant.TokenSet)
	require.Same(t, principal, minter.principal)
	require.Same(t, sess, minter.session)
	stored, err := store.GetRefreshToken(context.Background(), grant.TokenSet.RefreshToken.Value)
	require.NoError(t, err)
	require.Same(t, grant.TokenSet.RefreshToken, stored)
}

type sessionCreatorStub struct {
	session *sessiondomain.Session
}

func (s sessionCreatorStub) Create(context.Context, *authentication.Principal) (*sessiondomain.Session, error) {
	return s.session, nil
}

type recordingTokenPairMinter struct {
	set       *UserTokenSet
	principal *authentication.Principal
	session   *sessiondomain.Session
}

func (m *recordingTokenPairMinter) MintTokenSet(_ context.Context, principal *authentication.Principal, session *sessiondomain.Session) (*UserTokenSet, error) {
	m.principal = principal
	m.session = session
	return m.set, nil
}
