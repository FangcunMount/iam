package token

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
)

func TestRefresherConcurrentUseReturnsOnlyOneTokenPair(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "refresh-conflict.log")
	logOptions := log.NewOptions()
	logOptions.Format = "json"
	logOptions.OutputPaths = []string{logPath}
	logOptions.ErrorOutputPaths = []string{logPath}
	log.Init(logOptions)
	t.Cleanup(func() {
		log.Flush()
		log.Init(log.NewOptions())
	})

	const tokenSentinel = "refresh-conflict-secret-sentinel"
	old := testRefreshToken("old-id", tokenSentinel)
	store := newAtomicTokenStoreStub(old)
	revoker := &recordingSessionRevoker{}
	refresher := newRefresher(
		&atomicAccessTokenIssuerStub{},
		store,
		sessionLoaderStub{session: testActiveSession()},
		revoker,
		sessionExtenderStub{},
		subjectAccessEvaluatorStub{},
		NewDefaultRefreshClaimsCodec(),
	)

	start := make(chan struct{})
	type result struct {
		pair *TokenPair
		err  error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			pair, err := refresher.RefreshToken(context.Background(), old.Value)
			results <- result{pair: pair, err: err}
		}()
	}
	close(start)

	successes := 0
	notFound := 0
	for range 2 {
		got := <-results
		switch {
		case got.err == nil && got.pair != nil:
			successes++
		case perrors.IsCode(got.err, code.ErrRefreshTokenNotFound):
			notFound++
		default:
			t.Fatalf("RefreshToken() pair = %#v, err = %v", got.pair, got.err)
		}
	}
	if successes != 1 || notFound != 1 {
		t.Fatalf("successes = %d, not_found = %d, want 1/1", successes, notFound)
	}
	if got := revoker.count(); got != 1 {
		t.Fatalf("session revocations = %d, want 1 replay revocation", got)
	}
	log.Flush()
	output, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{tokenSentinel, "token_hint", "refresh_token:"} {
		if strings.Contains(string(output), forbidden) {
			t.Fatalf("refresh conflict log leaked %q: %s", forbidden, output)
		}
	}
}

func TestRefresherReplayedConsumedTokenRevokesSession(t *testing.T) {
	old := testRefreshToken("old-id", "consumed-value")
	store := newAtomicTokenStoreStub()
	store.consumed[old.Value] = &ConsumedRefreshToken{SessionID: old.SessionID, UserID: old.UserID}
	revoker := &recordingSessionRevoker{}
	refresher := newRefresher(
		&atomicAccessTokenIssuerStub{}, store, sessionLoaderStub{session: testActiveSession()},
		revoker, sessionExtenderStub{}, subjectAccessEvaluatorStub{}, NewDefaultRefreshClaimsCodec(),
	)

	pair, err := refresher.RefreshToken(context.Background(), old.Value)
	if pair != nil || !perrors.IsCode(err, code.ErrRefreshTokenNotFound) {
		t.Fatalf("RefreshToken() pair = %#v, err = %v, want not found", pair, err)
	}
	if got := revoker.count(); got != 1 {
		t.Fatalf("session revocations = %d, want 1", got)
	}
}

func TestRefresherUnknownTokenDoesNotRevokeSession(t *testing.T) {
	store := newAtomicTokenStoreStub()
	revoker := &recordingSessionRevoker{}
	refresher := newRefresher(
		&atomicAccessTokenIssuerStub{}, store, sessionLoaderStub{session: testActiveSession()},
		revoker, sessionExtenderStub{}, subjectAccessEvaluatorStub{}, NewDefaultRefreshClaimsCodec(),
	)

	pair, err := refresher.RefreshToken(context.Background(), "never-issued")
	if pair != nil || !perrors.IsCode(err, code.ErrRefreshTokenNotFound) {
		t.Fatalf("RefreshToken() pair = %#v, err = %v, want not found", pair, err)
	}
	if got := revoker.count(); got != 0 {
		t.Fatalf("session revocations = %d, want 0", got)
	}
}

func TestRefresherExtensionFailurePreservesOldToken(t *testing.T) {
	old := testRefreshToken("old-id", "old-value")
	store := newAtomicTokenStoreStub(old)
	refresher := newRefresher(
		&atomicAccessTokenIssuerStub{},
		store,
		sessionLoaderStub{session: testActiveSession()},
		sessionRevokerStub{},
		sessionExtenderStub{err: errors.New("session store unavailable")},
		subjectAccessEvaluatorStub{},
		NewDefaultRefreshClaimsCodec(),
	)

	pair, err := refresher.RefreshToken(context.Background(), old.Value)
	if err == nil || pair != nil {
		t.Fatalf("RefreshToken() pair = %#v, err = %v, want failure", pair, err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.tokens[old.Value] == nil {
		t.Fatal("old refresh token was removed after session extension failure")
	}
	if store.rotateCalls != 0 {
		t.Fatalf("RotateRefreshToken() calls = %d, want 0", store.rotateCalls)
	}
}

type atomicAccessTokenIssuerStub struct {
	mu   sync.Mutex
	next int
}

func (s *atomicAccessTokenIssuerStub) IssueToken(context.Context, *authentication.Principal) (*TokenPair, error) {
	return nil, errors.New("not implemented")
}

func (s *atomicAccessTokenIssuerStub) MintTokenPair(_ context.Context, principal *authentication.Principal, session *sessiondomain.Session) (*TokenPair, error) {
	s.mu.Lock()
	s.next++
	n := s.next
	s.mu.Unlock()
	access := NewAccessToken(
		meta.FromUint64(uint64(100+n)).String(),
		meta.FromUint64(uint64(200+n)).String(),
		session.SessionID,
		principal.UserID,
		principal.LoginIdentityID,
		principal.TenantID,
		time.Minute,
	)
	refresh := testRefreshToken(
		meta.FromUint64(uint64(300+n)).String(),
		meta.FromUint64(uint64(400+n)).String(),
	)
	return NewTokenPair(access, refresh), nil
}

type atomicTokenStoreStub struct {
	mu          sync.Mutex
	tokens      map[string]*Token
	consumed    map[string]*ConsumedRefreshToken
	rotateCalls int
}

func newAtomicTokenStoreStub(tokens ...*Token) *atomicTokenStoreStub {
	out := &atomicTokenStoreStub{
		tokens:   make(map[string]*Token),
		consumed: make(map[string]*ConsumedRefreshToken),
	}
	for _, token := range tokens {
		out.tokens[token.Value] = token
	}
	return out
}

func (s *atomicTokenStoreStub) GetConsumedRefreshToken(_ context.Context, value string) (*ConsumedRefreshToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.consumed[value], nil
}

func (s *atomicTokenStoreStub) SaveRefreshToken(_ context.Context, token *Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.Value] = token
	return nil
}

func (s *atomicTokenStoreStub) GetRefreshToken(_ context.Context, value string) (*Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokens[value], nil
}

func (s *atomicTokenStoreStub) RotateRefreshToken(_ context.Context, oldValue, expectedOldID string, newToken *Token) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rotateCalls++
	old := s.tokens[oldValue]
	if old == nil || old.ID != expectedOldID {
		return false, nil
	}
	s.tokens[newToken.Value] = newToken
	s.consumed[oldValue] = &ConsumedRefreshToken{SessionID: old.SessionID, UserID: old.UserID}
	delete(s.tokens, oldValue)
	return true, nil
}

func (s *atomicTokenStoreStub) DeleteRefreshToken(_ context.Context, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, value)
	return nil
}

func (*atomicTokenStoreStub) MarkAccessTokenRevoked(context.Context, string, time.Duration) error {
	return nil
}

func (*atomicTokenStoreStub) IsAccessTokenRevoked(context.Context, string) (bool, error) {
	return false, nil
}

type sessionLoaderStub struct {
	session *sessiondomain.Session
}

func (s sessionLoaderStub) Get(context.Context, string) (*sessiondomain.Session, error) {
	return s.session, nil
}

func (s sessionLoaderStub) GetActive(context.Context, string) (*sessiondomain.Session, error) {
	return s.session, nil
}

type sessionRevokerStub struct{}

func (sessionRevokerStub) Revoke(context.Context, string, string, string) error { return nil }
func (sessionRevokerStub) RevokeByUser(context.Context, meta.ID, string, string) error {
	return nil
}

type recordingSessionRevoker struct {
	mu      sync.Mutex
	revoked []string
}

func (r *recordingSessionRevoker) Revoke(_ context.Context, sessionID, _, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revoked = append(r.revoked, sessionID)
	return nil
}

func (*recordingSessionRevoker) RevokeByUser(context.Context, meta.ID, string, string) error {
	return nil
}

func (*recordingSessionRevoker) RevokeByLoginIdentity(context.Context, meta.ID, string, string) error {
	return nil
}

func (r *recordingSessionRevoker) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.revoked)
}
func (sessionRevokerStub) RevokeByLoginIdentity(context.Context, meta.ID, string, string) error {
	return nil
}

type sessionExtenderStub struct {
	err error
}

func (s sessionExtenderStub) Extend(context.Context, string, time.Time) error { return s.err }
func (s sessionExtenderStub) ExtendToRefreshExpiry(context.Context, *sessiondomain.Session, time.Time) error {
	return s.err
}

type subjectAccessEvaluatorStub struct{}

func (subjectAccessEvaluatorStub) Evaluate(context.Context, meta.ID, meta.ID) (sessiondomain.SubjectAccessDecision, error) {
	return sessiondomain.SubjectAccessDecision{Status: sessiondomain.SubjectAccessActive}, nil
}

func testRefreshToken(id, value string) *Token {
	token := NewRefreshToken(
		id, value, "session-id",
		meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3),
		nil, nil, time.Hour,
	)
	token.AuthMethod = "password"
	return token
}

func testActiveSession() *sessiondomain.Session {
	return sessiondomain.New(
		"session-id",
		meta.FromUint64(1),
		meta.FromUint64(2),
		meta.FromUint64(3),
		nil,
		nil,
		time.Now().Add(time.Hour),
	)
}
