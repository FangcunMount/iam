package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
)

func TestChallengeRepositoryCreateGetConsume(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	repo := NewChallengeRepository(client)
	ctx := context.Background()
	challenge := &challengeDomain.AuthChallenge{
		ID:         "sms_otp:login:+8613800138000",
		Type:       challengeDomain.TypeSMSOTP,
		Scene:      "login",
		Target:     "+8613800138000",
		SecretHash: []byte("hash"),
		ExpiresAt:  time.Now().Add(time.Minute),
		CreatedAt:  time.Now(),
	}

	if err := repo.Create(ctx, challenge); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	rawKey := challengeRedisKey(challenge.ID)
	if !mr.Exists(rawKey) {
		t.Fatalf("expected challenge key %q to exist", rawKey)
	}

	loaded, err := repo.Get(ctx, challenge.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded == nil || loaded.ID != challenge.ID {
		t.Fatalf("loaded challenge = %#v", loaded)
	}

	if err := repo.Consume(ctx, challenge.ID); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if mr.Exists(rawKey) {
		t.Fatalf("expected challenge key %q to be consumed", rawKey)
	}
}
