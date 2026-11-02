package main

import (
	"context"
	"testing"
	"time"

	sessiondomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/session"
	sessioncache "github.com/FangcunMount/iam/v5/internal/apiserver/infra/cache/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestExpiredApprovalDuringSessionScanDoesNotRevoke(t *testing.T) {
	m := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: m.Addr()})
	defer func() { _ = c.Close() }()
	store := sessioncache.NewSessionStore(c)
	s := &sessiondomain.Session{SessionID: "unchanged-after-approval-expiry", UserID: 101, LoginIdentityID: 301, Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.Save(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	before, err := c.Get(context.Background(), "session:"+s.SessionID).Result()
	if err != nil {
		t.Fatal(err)
	}
	c.AddHook(expiringScanHook{delay: 100 * time.Millisecond})
	now := time.Now()
	ctx, cancel := operationContext(options{action: "apply", deadline: 5 * time.Second}, approval{Expires: now.Add(40 * time.Millisecond), Deadline: now.Add(4 * time.Second)})
	defer cancel()
	_, err = inspectSessions(ctx, c, iamScope{[]string{"101"}, []string{}, []string{"301"}}, sessionOwnerImages([2]string{"301", "101"}), true)
	if err == nil {
		t.Fatal("approval expired during SCAN but session revocation still succeeded")
	}
	after, err := c.Get(context.Background(), "session:"+s.SessionID).Result()
	if err != nil || before != after {
		t.Fatal("expired approval changed the original session")
	}
}

func TestOperationContextUsesEarliestWriteDeadline(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name    string
		action  string
		cli     time.Duration
		expires time.Time
		window  time.Time
		want    time.Time
		wantCLI bool
	}{
		{"apply-expiry", "apply", time.Hour, now.Add(time.Minute), now.Add(2 * time.Minute), now.Add(time.Minute), false},
		{"restore-expiry", "restore", time.Hour, now.Add(time.Minute), now.Add(2 * time.Minute), now.Add(time.Minute), false},
		{"earlier-window-is-fail-closed", "apply", time.Hour, now.Add(2 * time.Minute), now.Add(time.Minute), now.Add(time.Minute), false},
		{"shorter-cli-is-not-extended", "apply", time.Second, now.Add(time.Minute), now.Add(2 * time.Minute), time.Time{}, true},
		{"readonly-has-no-write-approval", "capture", time.Second, time.Time{}, time.Time{}, time.Time{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now()
			ctx, cancel := operationContext(options{action: tc.action, deadline: tc.cli}, approval{Expires: tc.expires, Deadline: tc.window})
			defer cancel()
			after := time.Now()
			got, ok := ctx.Deadline()
			if !ok {
				t.Fatal("operation lost its fixed deadline")
			}
			if tc.wantCLI {
				if got.Before(before.Add(tc.cli)) || got.After(after.Add(tc.cli)) {
					t.Fatal("CLI limit was extended or replaced by an absent approval")
				}
			} else if !got.Equal(tc.want) {
				t.Fatal("write context did not inherit the earliest approved absolute instant")
			}
		})
	}
}

type expiringScanHook struct{ delay time.Duration }

func (h expiringScanHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h expiringScanHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h expiringScanHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "scan" {
			time.Sleep(h.delay)
		}
		return next(ctx, cmd)
	}
}
