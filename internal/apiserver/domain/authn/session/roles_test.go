package session

import (
	"context"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/stretchr/testify/require"
)

type lifecycleStoreStub struct {
	session           *Session
	extendedSessionID string
	extendedExpiresAt time.Time
}

func (s *lifecycleStoreStub) Save(_ context.Context, session *Session) error {
	s.session = session
	return nil
}

func (s *lifecycleStoreStub) Get(context.Context, string) (*Session, error) {
	return s.session, nil
}

func (s *lifecycleStoreStub) Revoke(context.Context, string, string, string) error {
	return nil
}

func (s *lifecycleStoreStub) Extend(_ context.Context, sessionID string, expiresAt time.Time) error {
	s.extendedSessionID = sessionID
	s.extendedExpiresAt = expiresAt
	if s.session != nil {
		s.session.ExpiresAt = expiresAt
	}
	return nil
}

func (s *lifecycleStoreStub) RevokeByUser(context.Context, meta.ID, string, string) error {
	return nil
}

func (s *lifecycleStoreStub) RevokeByLoginIdentity(context.Context, meta.ID, string, string) error {
	return nil
}

func TestCreatorCreateCapsInitialExpiryBySessionMaxTTL(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	store := &lifecycleStoreStub{}
	creator := NewCreator(store, NewLifetimePolicy(7*24*time.Hour, 24*time.Hour))

	session, err := creator.Create(context.Background(), &authentication.Principal{
		UserID:          meta.FromUint64(1),
		LoginIdentityID: meta.FromUint64(2),
		TenantID:        meta.FromUint64(3),
		AMR:             []string{"pwd"},
	})

	require.NoError(t, err)
	require.NotNil(t, session)
	require.Same(t, session, store.session)
	require.WithinDuration(t, now.Add(24*time.Hour), session.ExpiresAt, time.Second)
}

func TestLoaderGetActiveRejectsInactiveOrOverMaxLifetimeSession(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	tests := []struct {
		name    string
		session *Session
	}{
		{
			name:    "missing",
			session: nil,
		},
		{
			name:    "naturally expired",
			session: New("session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, now.Add(-time.Minute)),
		},
		{
			name: "revoked",
			session: func() *Session {
				session := New("session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, now.Add(time.Hour))
				session.Revoke("test", "test")
				return session
			}(),
		},
		{
			name: "past maximum lifetime",
			session: func() *Session {
				session := New("session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, now.Add(7*24*time.Hour))
				session.CreatedAt = now.Add(-25 * time.Hour)
				return session
			}(),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loader := NewLoader(&lifecycleStoreStub{session: tt.session}, NewLifetimePolicy(7*24*time.Hour, 24*time.Hour))

			session, err := loader.GetActive(context.Background(), "session-id")

			require.Error(t, err)
			require.Nil(t, session)
			require.Equal(t, code.ErrSessionInactive, perrors.ParseCoder(err).Code())
		})
	}
}

func TestRefreshExpirerNextRefreshExpiresAtCapsBySessionBoundary(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	session := New("session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, now.Add(7*24*time.Hour))
	session.CreatedAt = now.Add(-23 * time.Hour)
	refreshExpirer := NewRefreshExpirer(NewLifetimePolicy(7*24*time.Hour, 24*time.Hour))

	expiresAt, err := refreshExpirer.NextRefreshExpiresAt(now, session)

	require.NoError(t, err)
	require.Equal(t, session.CreatedAt.Add(24*time.Hour), expiresAt)
}

func TestExtenderExtendToRefreshExpiryCapsBySessionBoundary(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	session := New("session-id", meta.FromUint64(1), meta.FromUint64(2), meta.FromUint64(3), []string{"pwd"}, nil, now.Add(7*24*time.Hour))
	session.CreatedAt = now.Add(-23 * time.Hour)
	store := &lifecycleStoreStub{session: session}
	extender := NewExtender(store, NewLifetimePolicy(7*24*time.Hour, 24*time.Hour))

	err := extender.ExtendToRefreshExpiry(context.Background(), session, now.Add(7*24*time.Hour))

	require.NoError(t, err)
	require.Equal(t, "session-id", store.extendedSessionID)
	require.WithinDuration(t, session.CreatedAt.Add(24*time.Hour), store.extendedExpiresAt, time.Second)
}
