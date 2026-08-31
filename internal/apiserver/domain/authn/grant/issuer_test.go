package grant

import (
	"context"
	"errors"
	"testing"
	"time"

	admissiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/admission"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	tokendomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

func TestIssuerRequiresAdmissionBeforeCreatingAuthenticationState(t *testing.T) {
	t.Parallel()

	principal := testPrincipal()
	creator := &recordingSessionCreator{session: testSession(principal)}
	minter := &recordingTokenSetMinter{}
	saver := &recordingRefreshTokenSaver{}
	policy := admissionPolicyStub{decision: admissiondomain.Deny(
		admissiondomain.Subject{UserID: principal.UserID, LoginIdentityID: principal.LoginIdentityID},
		admissiondomain.ReasonUserBlocked,
	)}
	issuer := NewIssuer(Dependencies{
		AdmissionPolicy: policy, SessionCreator: creator, TokenSetMinter: minter, RefreshTokenSaver: saver,
	})

	result, err := issuer.Issue(context.Background(), principal)

	require.Nil(t, result)
	var denied *admissiondomain.DeniedError
	require.ErrorAs(t, err, &denied)
	require.Equal(t, admissiondomain.ReasonUserBlocked, denied.Decision.Reason)
	require.False(t, creator.called)
	require.False(t, minter.called)
	require.False(t, saver.called)
}

func TestIssuerDoesNotCreateAuthenticationStateWhenAdmissionCannotBeEvaluated(t *testing.T) {
	t.Parallel()

	principal := testPrincipal()
	creator := &recordingSessionCreator{session: testSession(principal)}
	issuer := NewIssuer(Dependencies{
		AdmissionPolicy: admissionPolicyStub{err: errors.New("status unavailable")},
		SessionCreator:  creator,
	})

	result, err := issuer.Issue(context.Background(), principal)

	require.Nil(t, result)
	var evaluation *admissiondomain.EvaluationError
	require.ErrorAs(t, err, &evaluation)
	require.False(t, creator.called)
}

func TestIssuerCreatesGrantAndPersistsInitialRefreshToken(t *testing.T) {
	t.Parallel()

	principal := testPrincipal()
	sess := testSession(principal)
	refresh := tokendomain.NewRefreshToken(
		"refresh-id", "refresh-value", sess.SessionID,
		principal.UserID, principal.LoginIdentityID, principal.TenantID,
		nil, nil, time.Hour,
	)
	set := tokendomain.NewUserTokenSet(
		tokendomain.NewAccessToken(
			"access-id", "access-value", sess.SessionID,
			principal.UserID, principal.LoginIdentityID, principal.TenantID,
			time.Minute,
		),
		refresh,
	)
	creator := &recordingSessionCreator{session: sess}
	minter := &recordingTokenSetMinter{set: set}
	saver := &recordingRefreshTokenSaver{}
	issuer := NewIssuer(Dependencies{
		AdmissionPolicy: admissionPolicyStub{decision: admissiondomain.Admit(
			admissiondomain.Subject{UserID: principal.UserID, LoginIdentityID: principal.LoginIdentityID},
		)},
		SessionCreator: creator, TokenSetMinter: minter, RefreshTokenSaver: saver,
	})

	result, err := issuer.Issue(context.Background(), principal)

	require.NoError(t, err)
	require.Same(t, sess, result.Session)
	require.Same(t, set, result.TokenSet)
	require.Same(t, principal, minter.principal)
	require.Same(t, sess, minter.session)
	require.Same(t, refresh, saver.token)
}

type admissionPolicyStub struct {
	decision admissiondomain.Decision
	err      error
}

func (s admissionPolicyStub) Evaluate(context.Context, admissiondomain.Subject) (admissiondomain.Decision, error) {
	return s.decision, s.err
}

type recordingSessionCreator struct {
	session *sessiondomain.Session
	called  bool
}

func (s *recordingSessionCreator) Create(context.Context, *authentication.Principal) (*sessiondomain.Session, error) {
	s.called = true
	return s.session, nil
}

type recordingTokenSetMinter struct {
	set       *tokendomain.UserTokenSet
	principal *authentication.Principal
	session   *sessiondomain.Session
	called    bool
}

func (m *recordingTokenSetMinter) MintTokenSet(_ context.Context, principal *authentication.Principal, session *sessiondomain.Session) (*tokendomain.UserTokenSet, error) {
	m.called = true
	m.principal = principal
	m.session = session
	return m.set, nil
}

type recordingRefreshTokenSaver struct {
	token  *tokendomain.RefreshToken
	called bool
}

func (s *recordingRefreshTokenSaver) SaveRefreshToken(_ context.Context, token *tokendomain.RefreshToken) error {
	s.called = true
	s.token = token
	return nil
}

func testPrincipal() *authentication.Principal {
	return &authentication.Principal{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
		TenantID:        meta.FromUint64(3),
	}
}

func testSession(principal *authentication.Principal) *sessiondomain.Session {
	return sessiondomain.New(
		"session-id", principal.UserID, principal.LoginIdentityID, principal.TenantID,
		nil, nil, time.Now().Add(time.Hour),
	)
}
