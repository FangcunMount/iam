package token_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type genStub struct {
	tok        *token.Token
	serviceTok *token.Token
	err        error
}

func (g *genStub) GenerateAccessToken(ctx context.Context, pr *authentication.Principal, expiresIn time.Duration) (*token.Token, error) {
	if g.err != nil {
		return nil, g.err
	}
	return g.tok, nil
}
func (g *genStub) GenerateServiceToken(ctx context.Context, subject string, audience []string, attributes map[string]string, expiresIn time.Duration) (*token.Token, error) {
	if g.err != nil {
		return nil, g.err
	}
	if g.serviceTok != nil {
		return g.serviceTok, nil
	}
	return g.tok, nil
}
func (g *genStub) ParseAccessToken(ctx context.Context, tokenValue string) (*token.TokenClaims, error) {
	if g.tok == nil {
		return nil, errors.New("no token")
	}
	// return claims built from g.tok
	return token.NewTokenClaims(g.tok.Type, g.tok.ID, g.tok.Subject, g.tok.SessionID, g.tok.UserID, g.tok.AccountID, g.tok.TenantID, "", g.tok.Audience, g.tok.Attributes, g.tok.AMR, g.tok.IssuedAt, g.tok.ExpiresAt), nil
}

type sessionStoreStub struct {
	sessions map[string]*sessiondomain.Session
}

func (s *sessionStoreStub) Save(_ context.Context, sess *sessiondomain.Session) error {
	if s.sessions == nil {
		s.sessions = make(map[string]*sessiondomain.Session)
	}
	s.sessions[sess.SessionID] = sess
	return nil
}

func (s *sessionStoreStub) Get(_ context.Context, sessionID string) (*sessiondomain.Session, error) {
	if s.sessions == nil {
		return nil, nil
	}
	return s.sessions[sessionID], nil
}

func (s *sessionStoreStub) Revoke(_ context.Context, sessionID string, reason string, revokedBy string) error {
	if s.sessions == nil {
		return nil
	}
	if sess, ok := s.sessions[sessionID]; ok {
		sess.Revoke(reason, revokedBy)
	}
	return nil
}

func (s *sessionStoreStub) Extend(_ context.Context, sessionID string, expiresAt time.Time) error {
	if s.sessions == nil {
		return nil
	}
	if sess, ok := s.sessions[sessionID]; ok {
		sess.Extend(expiresAt)
	}
	return nil
}

func (s *sessionStoreStub) RevokeByUser(_ context.Context, userID meta.ID, reason string, revokedBy string) error {
	return nil
}

func (s *sessionStoreStub) RevokeByAccount(_ context.Context, accountID meta.ID, reason string, revokedBy string) error {
	return nil
}

type storeStub struct {
	saved                    *token.Token
	revokedAccessTokenID     string
	revokedAccessTokenExpiry time.Duration
	saveErr                  error
	revokeMarkErr            error
	markCalled               int
}

func (s *storeStub) SaveRefreshToken(ctx context.Context, t *token.Token) error {
	s.saved = t
	return s.saveErr
}
func (s *storeStub) GetRefreshToken(ctx context.Context, tokenValue string) (*token.Token, error) {
	return nil, nil
}
func (s *storeStub) DeleteRefreshToken(ctx context.Context, tokenValue string) error { return nil }
func (s *storeStub) MarkAccessTokenRevoked(ctx context.Context, tokenID string, expiry time.Duration) error {
	s.markCalled++
	s.revokedAccessTokenID = tokenID
	s.revokedAccessTokenExpiry = expiry
	return s.revokeMarkErr
}
func (s *storeStub) IsAccessTokenRevoked(ctx context.Context, tokenID string) (bool, error) {
	return false, nil
}

func TestIssueToken_HappyPathAndGeneratorError(t *testing.T) {
	acc := &authentication.Principal{AccountID: meta.FromUint64(2), UserID: meta.FromUint64(1), TenantID: meta.FromUint64(0)}

	// happy path
	sessionManager := sessiondomain.NewManager(&sessionStoreStub{})
	access := token.NewAccessToken("aid", "aval", "sid-access", acc.UserID, acc.AccountID, acc.TenantID, time.Minute)
	gen := &genStub{tok: access}
	store := &storeStub{}
	issuer := token.NewTokenIssuer(gen, store, sessionManager, time.Minute, time.Hour)

	pair, err := issuer.IssueToken(context.Background(), acc)
	require.NoError(t, err)
	require.NotNil(t, pair)
	require.Equal(t, access.Value, pair.AccessToken.Value)
	require.NotNil(t, store.saved)
	require.Equal(t, pair.RefreshToken.UserID, acc.UserID)

	// generator error
	genErr := &genStub{err: errors.New("gen fail")}
	issuer2 := token.NewTokenIssuer(genErr, store, sessionManager, time.Minute, time.Hour)
	_, err2 := issuer2.IssueToken(context.Background(), acc)
	require.Error(t, err2)
}

func TestIssueServiceToken_HappyPathAndValidation(t *testing.T) {
	serviceTok := token.NewServiceToken("sid", "sval", "service:qs-server", []string{"iam-service"}, map[string]string{"scope": "internal"}, time.Minute)
	gen := &genStub{serviceTok: serviceTok}
	store := &storeStub{}
	issuer := token.NewTokenIssuer(gen, store, sessiondomain.NewManager(&sessionStoreStub{}), time.Minute, time.Hour)

	pair, err := issuer.IssueServiceToken(context.Background(), "service:qs-server", []string{"iam-service"}, map[string]string{"scope": "internal"}, 0)
	require.NoError(t, err)
	require.NotNil(t, pair)
	require.Equal(t, serviceTok.Value, pair.AccessToken.Value)
	require.Nil(t, pair.RefreshToken)
	require.Nil(t, store.saved)

	_, err = issuer.IssueServiceToken(context.Background(), "", []string{"iam-service"}, nil, time.Minute)
	require.Error(t, err)
}

func TestRevokeToken_ExpiredAndMarkRevokedAccessToken(t *testing.T) {
	// expired token: Parse returns claims with past expiry
	expired := token.NewAccessToken("eid", "eval", "sid-expired", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), -time.Minute)
	gen := &genStub{tok: expired}
	store := &storeStub{}
	issuer := token.NewTokenIssuer(gen, store, sessiondomain.NewManager(&sessionStoreStub{}), time.Minute, time.Hour)

	// Revoke should return nil when token already expired (no revocation marker)
	err := issuer.RevokeAccessToken(context.Background(), expired.Value)
	require.NoError(t, err)
	require.Equal(t, 0, store.markCalled)

	// future token -> revocation marker called
	futureTok := token.NewAccessToken("fid", "fval", "sid-future", meta.FromUint64(9), meta.FromUint64(8), meta.FromUint64(7), time.Hour)
	gen2 := &genStub{tok: futureTok}
	store2 := &storeStub{}
	issuer2 := token.NewTokenIssuer(gen2, store2, sessiondomain.NewManager(&sessionStoreStub{}), time.Minute, time.Hour)
	err2 := issuer2.RevokeAccessToken(context.Background(), futureTok.Value)
	require.NoError(t, err2)
	require.Equal(t, 1, store2.markCalled)
	require.Equal(t, futureTok.ID, store2.revokedAccessTokenID)
	// expiry should be close to RemainingDuration
	rem := futureTok.RemainingDuration()
	// allow small delta
	require.True(t, store2.revokedAccessTokenExpiry <= rem+time.Second && store2.revokedAccessTokenExpiry >= rem-time.Second)
}
