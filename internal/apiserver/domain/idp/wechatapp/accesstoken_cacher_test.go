package wechatapp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAccessTokenCacherEnsureTokenCacheHit(t *testing.T) {
	ctx := context.Background()
	app := NewWechatApp(MiniProgram, "app-1")
	cache := &accessTokenCacheStub{
		getResults: []*AppAccessToken{{Token: "cached", ExpiresAt: time.Now().Add(5 * time.Minute)}},
	}
	provider := &appTokenProviderStub{}

	token, err := NewAccessTokenCacher(cache, provider).EnsureToken(ctx, app, 120*time.Second)
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if token != "cached" {
		t.Fatalf("EnsureToken() token = %q, want cached", token)
	}
	if cache.lockCalls != 0 {
		t.Fatalf("lock calls = %d, want 0", cache.lockCalls)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
}

func TestAccessTokenCacherEnsureTokenLockAcquiredFetchesAndCaches(t *testing.T) {
	ctx := context.Background()
	app := NewWechatApp(MiniProgram, "app-1")
	cache := &accessTokenCacheStub{
		getResults: []*AppAccessToken{nil},
		lockOK:     true,
	}
	provider := &appTokenProviderStub{
		token: &AppAccessToken{Token: "fresh", ExpiresAt: time.Now().Add(30 * time.Second)},
	}

	token, err := NewAccessTokenCacher(cache, provider).EnsureToken(ctx, app, 0)
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if token != "fresh" {
		t.Fatalf("EnsureToken() token = %q, want fresh", token)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if cache.setCalls != 1 {
		t.Fatalf("set calls = %d, want 1", cache.setCalls)
	}
	if cache.setAppID != "app-1" {
		t.Fatalf("set appID = %q, want app-1", cache.setAppID)
	}
	if cache.setTTL != time.Minute {
		t.Fatalf("set TTL = %v, want 1m", cache.setTTL)
	}
	if !cache.unlocked {
		t.Fatalf("unlock was not called")
	}
}

func TestAccessTokenCacherEnsureTokenLockMissRereadsHit(t *testing.T) {
	ctx := context.Background()
	app := NewWechatApp(MiniProgram, "app-1")
	cache := &accessTokenCacheStub{
		getResults: []*AppAccessToken{
			nil,
			{Token: "reread", ExpiresAt: time.Now().Add(-time.Hour)},
		},
		lockOK: false,
	}
	provider := &appTokenProviderStub{}

	token, err := NewAccessTokenCacher(cache, provider).EnsureToken(ctx, app, 120*time.Second)
	if err != nil {
		t.Fatalf("EnsureToken() error = %v", err)
	}
	if token != "reread" {
		t.Fatalf("EnsureToken() token = %q, want reread", token)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
	if cache.getCalls != 2 {
		t.Fatalf("get calls = %d, want 2", cache.getCalls)
	}
}

func TestAccessTokenCacherEnsureTokenLockMissRereadsMiss(t *testing.T) {
	ctx := context.Background()
	app := NewWechatApp(MiniProgram, "app-1")
	cache := &accessTokenCacheStub{
		getResults: []*AppAccessToken{nil, nil},
		lockOK:     false,
	}
	provider := &appTokenProviderStub{}

	_, err := NewAccessTokenCacher(cache, provider).EnsureToken(ctx, app, 120*time.Second)
	if err == nil {
		t.Fatalf("EnsureToken() error = nil, want retry error")
	}
	if err.Error() != "access_token refresh in progress, please retry" {
		t.Fatalf("EnsureToken() error = %q, want retry error", err.Error())
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
}

type accessTokenCacheStub struct {
	getResults []*AppAccessToken
	getErrs    []error
	getCalls   int

	setCalls int
	setAppID string
	setToken *AppAccessToken
	setTTL   time.Duration
	setErr   error

	lockOK    bool
	lockErr   error
	lockCalls int
	unlocked  bool
}

func (s *accessTokenCacheStub) Get(context.Context, string) (*AppAccessToken, error) {
	call := s.getCalls
	s.getCalls++
	if call < len(s.getErrs) && s.getErrs[call] != nil {
		return nil, s.getErrs[call]
	}
	if call >= len(s.getResults) {
		return nil, nil
	}
	return s.getResults[call], nil
}

func (s *accessTokenCacheStub) Set(_ context.Context, appID string, aat *AppAccessToken, ttl time.Duration) error {
	s.setCalls++
	s.setAppID = appID
	s.setToken = aat
	s.setTTL = ttl
	return s.setErr
}

func (s *accessTokenCacheStub) TryLockRefresh(context.Context, string, time.Duration) (bool, func(), error) {
	s.lockCalls++
	if s.lockErr != nil {
		return false, nil, s.lockErr
	}
	return s.lockOK, func() {
		s.unlocked = true
	}, nil
}

type appTokenProviderStub struct {
	token *AppAccessToken
	err   error
	calls int
}

func (s *appTokenProviderStub) Fetch(context.Context, *WechatApp) (*AppAccessToken, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.token == nil {
		return nil, errors.New("missing token")
	}
	return s.token, nil
}
