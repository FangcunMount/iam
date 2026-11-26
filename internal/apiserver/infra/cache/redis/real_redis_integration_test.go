//go:build integration

package redis

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	challengeDomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/challenge"
	sessionDomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/session"
	tokenDomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/token"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// These contracts run only against an explicitly owned, disposable local Redis.
// Ordinary cloud unit tests keep using their self-contained miniredis fixtures.
func ownedRealRedis(t *testing.T) (*goredis.Client, context.Context) {
	t.Helper()
	addr := os.Getenv("IAM_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires disposable Redis: IAM_TEST_REDIS_ADDR and IAM_TEST_REDIS_OWNED=1")
	}
	require.Equal(t, "1", os.Getenv("IAM_TEST_REDIS_OWNED"), "must explicitly own the disposable Redis")
	host, _, err := net.SplitHostPort(addr)
	require.NoError(t, err, "Redis address must be host:port")
	ip := net.ParseIP(host)
	require.True(t, ip != nil && ip.IsLoopback(), "only literal loopback Redis endpoints are accepted")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	client := goredis.NewClient(&goredis.Options{
		Addr: addr, DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second,
	})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	require.NoError(t, client.Ping(ctx).Err())
	info, err := client.Info(ctx, "server").Result()
	require.NoError(t, err)
	require.Contains(t, info, "redis_version:7.", "this profile requires a real Redis 7 server")
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, "redis_version:") {
			t.Log(strings.TrimSpace(line))
		}
	}
	return client, ctx
}

func ownRealRedisKeys(t *testing.T, client *goredis.Client, keys ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	n, err := client.Exists(ctx, keys...).Result()
	require.NoError(t, err)
	require.Zero(t, n, "random fixture keys must not pre-exist")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, client.Del(ctx, keys...).Err())
		n, err := client.Exists(ctx, keys...).Result()
		require.NoError(t, err)
		require.Zero(t, n, "owned test keys must be removed")
		t.Log("OWNED_REDIS_KEYS_CLEANED", len(keys))
	})
}

func TestRealRedisSessionIndexesAndRevocation(t *testing.T) {
	client, ctx := ownedRealRedis(t)
	sess := newRedisTestSession("ci-" + uuid.NewString())
	sess.UserID = meta.FromUint64(uint64(time.Now().UnixNano()))
	sess.LoginIdentityID = meta.FromUint64(sess.UserID.Uint64() + 1)
	ownRealRedisKeys(t, client, sessionRedisKey(sess.SessionID),
		userSessionIndexRedisKey(sess.UserID.String()), loginIdentitySessionIndexRedisKey(sess.LoginIdentityID.String()))
	store := NewSessionStore(client)
	require.NoError(t, store.Save(ctx, sess))
	loaded, err := store.Get(ctx, sess.SessionID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, sess.UserID, loaded.UserID)
	requireSessionIndexed(t, client, sess)
	ttl, err := client.PTTL(ctx, sessionRedisKey(sess.SessionID)).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.NoError(t, store.Revoke(ctx, sess.SessionID, "ci-contract", "ci"))
	loaded, err = store.Get(ctx, sess.SessionID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, sessionDomain.StatusRevoked, loaded.Status)
	require.NotNil(t, loaded.RevokedAt)
	requireSessionNotIndexed(t, client, sess)
}

func TestRealRedisChallengeIsConsumedOnce(t *testing.T) {
	client, ctx := ownedRealRedis(t)
	challenge := &challengeDomain.AuthChallenge{
		ID: "ci-" + uuid.NewString(), Type: challengeDomain.TypeSMSOTP,
		SecretHash: []byte(uuid.NewString()), CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute),
	}
	ownRealRedisKeys(t, client, challengeRedisKey(challenge.ID), challengeAttemptRedisKey(challenge.ID, challenge.SecretHash))
	repo := NewChallengeRepository(client)
	require.NoError(t, repo.Create(ctx, challenge))
	consumed, err := repo.ConsumeIfSecretMatches(ctx, challenge.ID, []byte("stale-hash"))
	require.NoError(t, err)
	require.False(t, consumed)
	type result struct {
		consumed bool
		err      error
	}
	start := make(chan struct{})
	results := make(chan result, 8)
	for range cap(results) {
		go func() {
			<-start
			ok, err := repo.ConsumeIfSecretMatches(ctx, challenge.ID, challenge.SecretHash)
			results <- result{ok, err}
		}()
	}
	close(start)
	successes := 0
	for range cap(results) {
		got := <-results
		require.NoError(t, got.err)
		if got.consumed {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	remaining, err := repo.Get(ctx, challenge.ID)
	require.NoError(t, err)
	require.Nil(t, remaining)
}

func TestRealRedisRefreshTokenLifecycle(t *testing.T) {
	client, ctx := ownedRealRedis(t)
	id := "ci-" + uuid.NewString()
	token := tokenDomain.NewRefreshToken(id, uuid.NewString(), id,
		meta.FromUint64(1001), meta.FromUint64(2002), time.Now(), time.Now().Add(time.Minute))
	ownRealRedisKeys(t, client, refreshTokenRedisKey(token.Value))
	store := NewRedisStore(client)
	require.NoError(t, store.SaveRefreshToken(ctx, token))
	loaded, err := store.GetRefreshToken(ctx, token.Value)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, token.ID, loaded.ID)
	require.Equal(t, token.SessionID, loaded.SessionID)
	ttl, err := client.PTTL(ctx, refreshTokenRedisKey(token.Value)).Result()
	require.NoError(t, err)
	require.Positive(t, ttl)
	require.LessOrEqual(t, ttl, time.Minute)
	require.NoError(t, store.DeleteRefreshToken(ctx, token.Value))
	loaded, err = store.GetRefreshToken(ctx, token.Value)
	require.NoError(t, err)
	require.Nil(t, loaded)
}
