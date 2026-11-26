package main

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	keyspace "github.com/FangcunMount/component-base/pkg/redis/keyspace"
	sessiondomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/session"
	sessioncache "github.com/FangcunMount/iam/v5/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v5/internal/pkg/meta"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionRevocationIncludesUnindexedAndPreservesTTL(t *testing.T) {
	m := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: m.Addr()})
	defer func() { _ = c.Close() }()
	store := sessioncache.NewSessionStore(c)
	ctx := context.Background()
	expires := time.Now().Add(time.Hour).UTC()
	for _, s := range []*sessiondomain.Session{{SessionID: "indexed", UserID: meta.ID(101), LoginIdentityID: meta.ID(301), Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: expires}, {SessionID: "shared-kept", UserID: meta.ID(999), LoginIdentityID: meta.ID(999), Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: expires}} {
		if e := store.Save(ctx, s); e != nil {
			t.Fatal(e)
		}
	}
	// Deliberately absent from both ZSETs; the real legacy decoder and Store
	// revocation must still find and revoke it without rewriting another user.
	legacy := map[string]any{"SessionID": "unindexed", "UserID": uint64(101), "LoginIdentityID": uint64(301), "Status": "active", "CreatedAt": time.Now(), "ExpiresAt": expires}
	b, _ := json.Marshal(legacy)
	if e := c.Set(ctx, keyspace.New("session").Prefix("unindexed"), b, time.Hour).Err(); e != nil {
		t.Fatal(e)
	}
	scope := iamScope{[]string{"101"}, []string{}, []string{"301"}}
	images := sessionOwnerImages([2]string{"301", "101"})
	// A stale target index pointing at a retained user's session must never be
	// used as ownership evidence for bulk revocation.
	if e := c.ZAdd(ctx, keyspace.New("user_session_index").Prefix("101"), redis.Z{Score: float64(expires.Unix()), Member: "shared-kept"}).Err(); e != nil {
		t.Fatal(e)
	}
	if e := c.ZAdd(ctx, keyspace.New("login_identity_session_index").Prefix("301"), redis.Z{Score: float64(expires.Unix()), Member: "shared-kept"}).Err(); e != nil {
		t.Fatal(e)
	}
	before, e := inspectSessions(ctx, c, scope, images, false)
	if e != nil || len(before) != 2 {
		t.Fatalf("preflight %d %v", len(before), e)
	}
	if _, e = inspectSessions(ctx, c, scope, images, true); e != nil {
		t.Fatal(e)
	}
	for _, sid := range []string{"indexed", "unindexed"} {
		s, e := store.Get(ctx, sid)
		if e != nil || s == nil || s.IsActive() || s.Status != sessiondomain.StatusRevoked || !s.ExpiresAt.Equal(expires) {
			t.Fatalf("revocation/TTL mismatch %s %#v %v", sid, s, e)
		}
		ttl := m.TTL(keyspace.New("session").Prefix(sid))
		if ttl <= 0 || ttl > time.Hour {
			t.Fatal("revoked record retention lost")
		}
	}
	kept, e := store.Get(ctx, "shared-kept")
	if e != nil || !kept.IsActive() {
		t.Fatal("shared session changed")
	}
	if members, e := c.ZRange(ctx, keyspace.New("user_session_index").Prefix("101"), 0, -1).Result(); e != nil || len(members) != 1 || members[0] != "shared-kept" {
		t.Fatal("target session membership not removed or foreign member changed")
	}
	// A DB restore has no route to Save/Extend these sessions. Reinspection is
	// purely read-only and cannot reactivate the retained revoked payloads.
	if _, e = inspectSessions(ctx, c, scope, images, false); e != nil {
		t.Fatal(e)
	}
	for _, sid := range []string{"indexed", "unindexed"} {
		s, _ := store.Get(ctx, sid)
		if s.IsActive() {
			t.Fatal("readback revived session")
		}
	}
}
func TestRedisIdentityBindsRunAndLogicalDatabase(t *testing.T) {
	a, e := redisIdentityFromInfo("# Server\r\nrun_id:0123456789012345678901234567890123456789\r\n", 0)
	if e != nil {
		t.Fatal(e)
	}
	b, e := redisIdentityFromInfo("run_id:0123456789012345678901234567890123456789\n", 1)
	if e != nil || a == b {
		t.Fatal("Redis DB binding absent")
	}
	for _, info := range []string{"", "run_id:bad", "run_id:0123456789012345678901234567890123456789\nrun_id:0123456789012345678901234567890123456789"} {
		if _, e = redisIdentityFromInfo(info, 0); e == nil {
			t.Fatal("unknown Redis identity accepted")
		}
	}
}

func TestSessionRevocationRejectsMixedTypedOwnershipBeforeAnyWrite(t *testing.T) {
	for _, tc := range []struct {
		name        string
		user, login meta.ID
	}{
		{"retained-user-selected-login", 999, 301},
		{"selected-user-retained-login", 101, 999},
		{"selected-identities-wrong-captured-pair", 101, 302},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := miniredis.RunT(t)
			c := redis.NewClient(&redis.Options{Addr: m.Addr()})
			defer func() { _ = c.Close() }()
			ctx := context.Background()
			store := sessioncache.NewSessionStore(c)
			expires := time.Now().UTC().Add(time.Hour)
			for _, s := range []*sessiondomain.Session{
				{SessionID: "a-valid-target", UserID: 101, LoginIdentityID: 301, Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: expires},
				{SessionID: "z-conflicting-owner", UserID: tc.user, LoginIdentityID: tc.login, Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: expires},
			} {
				if err := store.Save(ctx, s); err != nil {
					t.Fatal(err)
				}
			}
			before, err := c.MGet(ctx, "session:a-valid-target", "session:z-conflicting-owner").Result()
			if err != nil {
				t.Fatal(err)
			}
			scope := iamScope{[]string{"101", "102"}, []string{}, []string{"301", "302"}}
			images := sessionOwnerImages([2]string{"301", "101"}, [2]string{"302", "102"})
			if _, err = inspectSessions(ctx, c, scope, images, true); err == nil {
				t.Fatal("mixed typed session ownership was admitted")
			}
			after, err := c.MGet(ctx, "session:a-valid-target", "session:z-conflicting-owner").Result()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("preflight rejection changed a target or retained session")
			}
		})
	}
}

func TestSessionRevocationRequiresCompleteCapturedLoginOwners(t *testing.T) {
	for _, tc := range []struct {
		name   string
		owners [][2]string
	}{
		{"missing", nil},
		{"foreign-user", [][2]string{{"301", "999"}}},
		{"duplicate-login", [][2]string{{"301", "101"}, {"301", "101"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := miniredis.RunT(t)
			c := redis.NewClient(&redis.Options{Addr: m.Addr()})
			defer func() { _ = c.Close() }()
			ctx := context.Background()
			store := sessioncache.NewSessionStore(c)
			s := &sessiondomain.Session{SessionID: "kept-until-ownership-proven", UserID: 101, LoginIdentityID: 301, Status: sessiondomain.StatusActive, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
			if err := store.Save(ctx, s); err != nil {
				t.Fatal(err)
			}
			raw, err := c.Get(ctx, "session:"+s.SessionID).Result()
			if err != nil {
				t.Fatal(err)
			}
			if _, err = inspectSessions(ctx, c, iamScope{[]string{"101"}, []string{}, []string{"301"}}, sessionOwnerImages(tc.owners...), true); err == nil {
				t.Fatal("incomplete captured ownership admitted session mutation")
			}
			if after, er := c.Get(ctx, "session:"+s.SessionID).Result(); er != nil || raw != after {
				t.Fatal("session changed without complete captured ownership")
			}
		})
	}
}

func sessionOwnerImages(pairs ...[2]string) []tableImage {
	im := tableImage{Name: "auth_login_identities", Columns: []column{{Name: "id"}, {Name: "user_id"}}, Rows: []row{}}
	for _, pair := range pairs {
		im.Rows = append(im.Rows, row{Values: []cell{{Raw: []byte(pair[0])}, {Raw: []byte(pair[1])}}})
	}
	return []tableImage{im}
}
