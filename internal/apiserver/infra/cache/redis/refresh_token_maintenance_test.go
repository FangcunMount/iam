package redis

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

type unlinkFailureHook struct{}

func (unlinkFailureHook) DialHook(next goredis.DialHook) goredis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (unlinkFailureHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		if cmd.Name() == "unlink" {
			return errors.New("unlink failure sentinel")
		}
		return next(ctx, cmd)
	}
}

func (unlinkFailureHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}

func TestPurgeRefreshTokensDryRunAndApply(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	for _, key := range []string{
		refreshTokenRedisKey("one"),
		refreshTokenRedisKey("two"),
		"session:keep",
		"challenge:keep",
	} {
		if err := client.Set(ctx, key, "value", 0).Err(); err != nil {
			t.Fatal(err)
		}
	}

	dryRun, err := PurgeRefreshTokens(ctx, client, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.Matched != 2 || dryRun.Deleted != 0 {
		t.Fatalf("dry-run result = %+v", dryRun)
	}
	if !mr.Exists(refreshTokenRedisKey("one")) {
		t.Fatal("dry-run deleted a refresh token")
	}

	applied, err := PurgeRefreshTokens(ctx, client, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Matched != 2 || applied.Deleted != 2 {
		t.Fatalf("apply result = %+v", applied)
	}
	for _, key := range []string{"session:keep", "challenge:keep"} {
		if !mr.Exists(key) {
			t.Fatalf("purge removed unrelated key %q", key)
		}
	}

	repeated, err := PurgeRefreshTokens(ctx, client, 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.Matched != 0 || repeated.Deleted != 0 {
		t.Fatalf("repeated result = %+v", repeated)
	}
}

func TestPurgeRefreshTokensValidatesBatchSize(t *testing.T) {
	_, err := PurgeRefreshTokens(context.Background(), nil, 0, false)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestPurgeRefreshTokensFailsClosedOnUnlinkError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	client.AddHook(unlinkFailureHook{})
	t.Cleanup(func() { _ = client.Close() })
	if err := mr.Set(refreshTokenRedisKey("keep-on-failure"), "value"); err != nil {
		t.Fatal(err)
	}

	result, err := PurgeRefreshTokens(context.Background(), client, 10, true)
	if err == nil {
		t.Fatal("expected UNLINK failure")
	}
	if result.Deleted != 0 || !mr.Exists(refreshTokenRedisKey("keep-on-failure")) {
		t.Fatalf("unlink failure produced partial deletion: %+v", result)
	}
}

func TestPurgeRefreshTokensHonorsCanceledContext(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := PurgeRefreshTokens(ctx, client, 10, false); err == nil {
		t.Fatal("expected canceled context")
	}
}
