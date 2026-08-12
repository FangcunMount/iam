package redis

import (
	"context"
	"sync"
	"testing"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSessionStoreSaveAndRevokeAreIndexConsistent(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewSessionStore(client)
	ctx := context.Background()
	sess := newRedisTestSession("sid-consistent")

	require.NoError(t, store.Save(ctx, sess))
	require.True(t, mr.Exists(sessionRedisKey(sess.SessionID)))
	requireSessionIndexed(t, client, sess)

	require.NoError(t, store.Revoke(ctx, sess.SessionID, "test", "tester"))
	loaded, err := store.Get(ctx, sess.SessionID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, session.StatusRevoked, loaded.Status)
	require.NotNil(t, loaded.RevokedAt)
	requireSessionNotIndexed(t, client, sess)
}

func TestSessionStoreRevokeWinsConcurrentExtend(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewSessionStore(client)
	ctx := context.Background()
	sess := newRedisTestSession("sid-revoke-wins")
	require.NoError(t, store.Save(ctx, sess))

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		errs <- store.Revoke(ctx, sess.SessionID, "security", "tester")
	}()
	go func() {
		defer wg.Done()
		<-start
		errs <- store.Extend(ctx, sess.SessionID, time.Now().Add(2*time.Hour))
	}()
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			require.True(t, perrors.IsCode(err, code.ErrSessionInactive), "unexpected error: %v", err)
		}
	}
	loaded, err := store.Get(ctx, sess.SessionID)
	require.NoError(t, err)
	require.Equal(t, session.StatusRevoked, loaded.Status)
	requireSessionNotIndexed(t, client, sess)
}

func TestSessionStoreExtendRejectsRevokedSession(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewSessionStore(client)
	ctx := context.Background()
	sess := newRedisTestSession("sid-inactive")
	require.NoError(t, store.Save(ctx, sess))
	require.NoError(t, store.Revoke(ctx, sess.SessionID, "security", "tester"))

	err := store.Extend(ctx, sess.SessionID, time.Now().Add(2*time.Hour))
	require.Error(t, err)
	require.True(t, perrors.IsCode(err, code.ErrSessionInactive))
	requireSessionNotIndexed(t, client, sess)
}

func TestSessionStoreBulkRevokeCleansStaleIndexMember(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewSessionStore(client)
	ctx := context.Background()
	userID := meta.FromUint64(1001)
	indexKey := userSessionIndexRedisKey(userID.String())
	require.NoError(t, client.ZAdd(ctx, indexKey, goredis.Z{
		Score:  float64(time.Now().Add(time.Hour).Unix()),
		Member: "missing-session",
	}).Err())

	require.NoError(t, store.RevokeByUser(ctx, userID, "test", "tester"))
	_, err := client.ZScore(ctx, indexKey, "missing-session").Result()
	require.ErrorIs(t, err, goredis.Nil)
}

func newRedisTestSession(id string) *session.Session {
	return session.New(
		id,
		meta.FromUint64(1001),
		meta.FromUint64(2001),
		meta.FromUint64(3001),
		[]string{"pwd"},
		map[string]string{"scope": "test"},
		time.Now().Add(time.Hour),
	)
}

func requireSessionIndexed(t *testing.T, client *goredis.Client, sess *session.Session) {
	t.Helper()
	ctx := context.Background()
	_, err := client.ZScore(ctx, userSessionIndexRedisKey(sess.UserID.String()), sess.SessionID).Result()
	require.NoError(t, err)
	_, err = client.ZScore(ctx, loginIdentitySessionIndexRedisKey(sess.LoginIdentityID.String()), sess.SessionID).Result()
	require.NoError(t, err)
}

func requireSessionNotIndexed(t *testing.T, client *goredis.Client, sess *session.Session) {
	t.Helper()
	ctx := context.Background()
	_, err := client.ZScore(ctx, userSessionIndexRedisKey(sess.UserID.String()), sess.SessionID).Result()
	require.ErrorIs(t, err, goredis.Nil)
	_, err = client.ZScore(ctx, loginIdentitySessionIndexRedisKey(sess.LoginIdentityID.String()), sess.SessionID).Result()
	require.ErrorIs(t, err, goredis.Nil)
}
