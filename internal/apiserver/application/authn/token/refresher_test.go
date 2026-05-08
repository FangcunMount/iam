package token

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v2/internal/pkg/code"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type refreshOrderRecorder struct {
	events []string
}

func (r *refreshOrderRecorder) record(event string) {
	r.events = append(r.events, event)
}

type refreshOrderStore struct {
	recorder     *refreshOrderRecorder
	refreshToken *Token
}

func (s *refreshOrderStore) SaveRefreshToken(context.Context, *Token) error {
	s.recorder.record("save_refresh")
	return nil
}

func (s *refreshOrderStore) GetRefreshToken(context.Context, string) (*Token, error) {
	s.recorder.record("load_refresh")
	return s.refreshToken, nil
}

func (s *refreshOrderStore) DeleteRefreshToken(context.Context, string) error {
	s.recorder.record("delete_refresh")
	return nil
}

func (s *refreshOrderStore) MarkAccessTokenRevoked(context.Context, string, time.Duration) error {
	s.recorder.record("mark_access_revoked")
	return nil
}

func (s *refreshOrderStore) IsAccessTokenRevoked(context.Context, string) (bool, error) {
	return false, nil
}

type refreshOrderSessionManager struct {
	recorder *refreshOrderRecorder
	session  *sessiondomain.Session
}

func (s *refreshOrderSessionManager) Create(context.Context, *authentication.Principal, time.Time) (*sessiondomain.Session, error) {
	s.recorder.record("create_session")
	return s.session, nil
}

func (s *refreshOrderSessionManager) Get(context.Context, string) (*sessiondomain.Session, error) {
	s.recorder.record("load_session")
	return s.session, nil
}

func (s *refreshOrderSessionManager) Revoke(context.Context, string, string, string) error {
	s.recorder.record("revoke_session")
	return nil
}

func (s *refreshOrderSessionManager) RevokeByUser(context.Context, meta.ID, string, string) error {
	s.recorder.record("revoke_user_sessions")
	return nil
}

func (s *refreshOrderSessionManager) RevokeByAccount(context.Context, meta.ID, string, string) error {
	s.recorder.record("revoke_account_sessions")
	return nil
}

func (s *refreshOrderSessionManager) Extend(context.Context, string, time.Time) error {
	s.recorder.record("extend_session")
	return nil
}

type refreshOrderAccessChecker struct {
	recorder *refreshOrderRecorder
}

func (s *refreshOrderAccessChecker) Evaluate(context.Context, meta.ID, meta.ID) (sessiondomain.SubjectAccessDecision, error) {
	s.recorder.record("evaluate_access")
	return sessiondomain.SubjectAccessDecision{Status: sessiondomain.SubjectAccessActive}, nil
}

type refreshOrderPairIssuer struct {
	recorder *refreshOrderRecorder
	pair     *TokenPair
}

func (s *refreshOrderPairIssuer) IssueTokenPair(context.Context, *authentication.Principal, *sessiondomain.Session) (*TokenPair, error) {
	s.recorder.record("issue_pair")
	if s.pair != nil {
		return s.pair, nil
	}
	return NewTokenPair(
		NewAccessToken("new-access", "new-access-value", "session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), time.Minute),
		NewRefreshToken("new-refresh", "new-refresh-value", "session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, time.Hour),
	), nil
}

func newRefreshOrderFixture(recorder *refreshOrderRecorder, pairIssuer SessionTokenPairIssuer) (Refresher, *Token) {
	refreshToken := NewRefreshToken(
		"refresh-id",
		"old-refresh-value",
		"session-id",
		meta.FromUint64(1),
		meta.FromUint64(2),
		meta.FromUint64(3),
		[]string{"pwd"},
		map[string]string{"auth_method": "password"},
		time.Hour,
	)
	sess := sessiondomain.New("session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, time.Now().Add(time.Hour))
	refresher := NewRefresher(
		pairIssuer,
		&refreshOrderStore{recorder: recorder, refreshToken: refreshToken},
		&refreshOrderSessionManager{recorder: recorder, session: sess},
		&refreshOrderAccessChecker{recorder: recorder},
		NewStringClaimMapper(),
	)
	return refresher, refreshToken
}

func TestRefresherRefreshTokenKeepsRotationOrder(t *testing.T) {
	t.Parallel()

	recorder := &refreshOrderRecorder{}
	refresher, _ := newRefreshOrderFixture(recorder, &refreshOrderPairIssuer{recorder: recorder})

	pair, err := refresher.RefreshToken(context.Background(), "old-refresh-value")

	require.NoError(t, err)
	require.NotNil(t, pair)
	require.Equal(t, []string{
		"load_refresh",
		"load_session",
		"evaluate_access",
		"issue_pair",
		"delete_refresh",
		"extend_session",
	}, recorder.events)
}

func TestRefresherRefreshTokenRejectsIncompleteIssuedPair(t *testing.T) {
	t.Parallel()

	recorder := &refreshOrderRecorder{}
	incompletePair := NewTokenPair(
		NewAccessToken("new-access", "new-access-value", "session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), time.Minute),
		nil,
	)
	refresher, _ := newRefreshOrderFixture(recorder, &refreshOrderPairIssuer{
		recorder: recorder,
		pair:     incompletePair,
	})

	pair, err := refresher.RefreshToken(context.Background(), "old-refresh-value")

	require.Error(t, err)
	require.Nil(t, pair)
	require.True(t, perrors.IsCode(err, code.ErrInternalServerError))
	require.Equal(t, []string{
		"load_refresh",
		"load_session",
		"evaluate_access",
		"issue_pair",
	}, recorder.events)
}
