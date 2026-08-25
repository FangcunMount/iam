package token

import (
	"context"
	"testing"
	"time"

	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/stretchr/testify/require"
)

func TestSessionEstablisherCreatesSessionAndPersistsInitialRefreshToken(t *testing.T) {
	t.Parallel()

	sess := testActiveSession()
	creator := sessionCreatorStub{session: sess}
	store := newAtomicTokenStoreStub()
	minter := &recordingTokenPairMinter{
		pair: NewTokenPair(
			NewAccessToken("access-id", "access-value", sess.SessionID, sess.UserID, sess.LoginIdentityID, sess.TenantID, time.Minute),
			testRefreshToken("refresh-id", "refresh-value"),
		),
	}
	principal := &authentication.Principal{
		UserID:          sess.UserID,
		LoginIdentityID: sess.LoginIdentityID,
		TenantID:        sess.TenantID,
	}

	pair, err := newSessionEstablisher(creator, store, minter).EstablishSession(context.Background(), principal)

	require.NoError(t, err)
	require.Same(t, minter.pair, pair)
	require.Same(t, principal, minter.principal)
	require.Same(t, sess, minter.session)
	stored, err := store.GetRefreshToken(context.Background(), pair.RefreshToken.Value)
	require.NoError(t, err)
	require.Same(t, pair.RefreshToken, stored)
}

type sessionCreatorStub struct {
	session *sessiondomain.Session
}

func (s sessionCreatorStub) Create(context.Context, *authentication.Principal) (*sessiondomain.Session, error) {
	return s.session, nil
}

type recordingTokenPairMinter struct {
	pair      *TokenPair
	principal *authentication.Principal
	session   *sessiondomain.Session
}

func (m *recordingTokenPairMinter) MintTokenPair(_ context.Context, principal *authentication.Principal, session *sessiondomain.Session) (*TokenPair, error) {
	m.principal = principal
	m.session = session
	return m.pair, nil
}
