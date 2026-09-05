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
		AdmissionPolicy: policy, SessionCreator: creator, SessionRevoker: &recordingSessionRevoker{}, TokenSetMinter: minter, RefreshTokenSaver: saver,
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
		SessionCreator: creator, SessionRevoker: &recordingSessionRevoker{}, TokenSetMinter: minter, RefreshTokenSaver: saver,
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
	err       error
	set       *tokendomain.UserTokenSet
	principal *authentication.Principal
	session   *sessiondomain.Session
	called    bool
}

func (m *recordingTokenSetMinter) MintTokenSet(_ context.Context, principal *authentication.Principal, session *sessiondomain.Session) (*tokendomain.UserTokenSet, error) {
	m.called = true
	m.principal = principal
	m.session = session
	return m.set, m.err
}

type recordingRefreshTokenSaver struct {
	err    error
	token  *tokendomain.RefreshToken
	called bool
}

func (s *recordingRefreshTokenSaver) SaveRefreshToken(_ context.Context, token *tokendomain.RefreshToken) error {
	s.called = true
	s.token = token
	return s.err
}

func testPrincipal() *authentication.Principal {
	return &authentication.Principal{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
		TenantID:        meta.FromUint64(3),
	}
}

func testSession(principal *authentication.Principal) *sessiondomain.Session {
	return sessiondomain.NewWithContexts(
		"session-id", principal.UserID, principal.LoginIdentityID, principal.TenantID,
		principal.AuthContext, principal.TokenContext, time.Now().Add(time.Hour),
	)
}

type recordingSessionRevoker struct {
	sessionID, reason string
	err               error
	contextErr        error
	deadline          bool
}

func (r *recordingSessionRevoker) Revoke(ctx context.Context, sid, reason, _ string) error {
	r.sessionID, r.reason = sid, reason
	r.contextErr = ctx.Err()
	_, r.deadline = ctx.Deadline()
	return r.err
}

func TestIssuerCompensatesFailedGrantEvenAfterRequestCancellation(t *testing.T) {
	for _, stage := range []string{"mint", "save", "incomplete"} {
		t.Run(stage, func(t *testing.T) {
			principal := testPrincipal()
			sess := testSession(principal)
			failure := errors.New("injected grant failure")
			minter := &recordingTokenSetMinter{set: &tokendomain.UserTokenSet{
				AccessToken: &tokendomain.AccessToken{}, RefreshToken: &tokendomain.RefreshToken{},
			}}
			saver := &recordingRefreshTokenSaver{}
			switch stage {
			case "mint":
				minter.err = failure
			case "save":
				saver.err = failure
			case "incomplete":
				minter.set = nil
			}
			for _, cleanupFails := range []bool{false, true} {
				revoker := &recordingSessionRevoker{}
				if cleanupFails {
					revoker.err = errors.New("injected cleanup failure")
				}
				issuer := NewIssuer(Dependencies{
					AdmissionPolicy: admissionPolicyStub{decision: admissiondomain.Admit(admissiondomain.Subject{})},
					SessionCreator:  &recordingSessionCreator{session: sess}, SessionRevoker: revoker,
					TokenSetMinter: minter, RefreshTokenSaver: saver,
				})
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				result, err := issuer.Issue(ctx, principal)
				require.Nil(t, result)
				require.Error(t, err)
				if stage != "incomplete" {
					require.ErrorIs(t, err, failure)
				}
				if cleanupFails {
					require.ErrorIs(t, err, revoker.err)
				}
				require.Equal(t, sess.SessionID, revoker.sessionID)
				require.Equal(t, "authentication_grant_failed", revoker.reason)
				require.NoError(t, revoker.contextErr)
				require.True(t, revoker.deadline)
			}
		})
	}
}

func TestIssuerRequiresCompensationBeforeCreatingSession(t *testing.T) {
	creator := &recordingSessionCreator{}
	issuer := NewIssuer(Dependencies{
		AdmissionPolicy: admissionPolicyStub{decision: admissiondomain.Admit(admissiondomain.Subject{})},
		SessionCreator:  creator, TokenSetMinter: &recordingTokenSetMinter{}, RefreshTokenSaver: &recordingRefreshTokenSaver{},
	})
	_, err := issuer.Issue(context.Background(), testPrincipal())
	require.Error(t, err)
	require.False(t, creator.called)
}
