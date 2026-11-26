package main

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	keyspace "github.com/FangcunMount/component-base/pkg/redis/keyspace"
	sessionapp "github.com/FangcunMount/iam/v5/internal/apiserver/application/authn/session"
	sessiondomain "github.com/FangcunMount/iam/v5/internal/apiserver/domain/authn/session"
	sessioncache "github.com/FangcunMount/iam/v5/internal/apiserver/infra/cache/redis"
	"github.com/redis/go-redis/v9"
)

type sessionEvidence struct {
	ID            string    `json:"session_id"`
	User          string    `json:"user_id"`
	LoginIdentity string    `json:"login_identity_id"`
	Expires       time.Time `json:"original_expires_at"`
	Status        string    `json:"status"`
}

func redisIdentity(ctx context.Context, c *redis.Client) (string, error) {
	info, err := c.Info(ctx, "server").Result()
	if err != nil {
		return "", err
	}
	return redisIdentityFromInfo(info, c.Options().DB)
}
func redisIdentityFromInfo(info string, db int) (string, error) {
	runID := ""
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, "run_id:") {
			if runID != "" {
				return "", errors.New("multiple Redis server identities")
			}
			runID = strings.TrimSpace(strings.TrimPrefix(line, "run_id:"))
		}
	}
	if len(runID) != 40 || db < 0 {
		return "", errors.New("Redis run identity unavailable")
	}
	if _, err := strconv.ParseUint(runID[:16], 16, 64); err != nil {
		return "", errors.New("Redis run identity invalid")
	}
	return rawSHA([]byte(runID + "\x00" + strconv.Itoa(db))), nil
}

func inspectSessions(ctx context.Context, c *redis.Client, scope iamScope, images []tableImage, revoke bool) ([]sessionEvidence, error) {
	store := sessioncache.NewSessionStore(c)
	revoker := sessionapp.NewRevoker(sessiondomain.NewRevoker(store))
	// Never bulk-revoke from an index: stale members may belong to retained users.
	users, logins := idSet(scope.Users), idSet(scope.LoginIdentities)
	owners := map[string]string{}
	im := lookup(images, "auth_login_identities")
	for _, r := range im.Rows {
		login, user := field(im, r, "id"), field(im, r, "user_id")
		lid, uid := string(login.Raw), string(user.Raw)
		if login.Null || user.Null || !logins[lid] || !users[uid] || owners[lid] != "" {
			return nil, errors.New("captured session login ownership incomplete")
		}
		owners[lid] = uid
	}
	if len(owners) != len(logins) {
		return nil, errors.New("captured session login ownership incomplete")
	}
	owned := func(sid string, s *sessiondomain.Session) (bool, error) {
		if s == nil {
			return false, nil
		}
		uid, lid := s.UserID.String(), s.LoginIdentityID.String()
		if !users[uid] && !logins[lid] {
			return false, nil
		}
		if !users[uid] || !logins[lid] || owners[lid] != uid || s.SessionID != sid {
			return false, errors.New("session typed ownership conflicts with captured cohort")
		}
		return true, nil
	}

	// Index-only revocation misses a historical unindexed session. SCAN is bounded
	// and entirely read-only until every target's two identities have been checked.
	prefix := keyspace.New("session").Prefix("") + ":"
	var cursor uint64
	seen := map[string]bool{}
	out := []sessionEvidence{}
	scanned := 0
	for {
		keys, next, e := c.Scan(ctx, cursor, prefix+"*", 500).Result()
		if e != nil {
			return nil, e
		}
		scanned += len(keys)
		if scanned > 2_000_000 {
			return nil, errors.New("session scan bound exceeded")
		}
		for _, key := range keys {
			sid := strings.TrimPrefix(key, prefix)
			if seen[sid] {
				continue
			}
			seen[sid] = true
			s, e := store.Get(ctx, sid)
			if e != nil {
				return nil, e
			}
			match, e := owned(sid, s)
			if e != nil {
				return nil, e
			}
			if !match {
				continue
			}
			out = append(out, sessionEvidence{sid, safeUser(s), safeLogin(s), s.ExpiresAt, string(s.Status)})
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	if revoke {
		for i, x := range out {
			s, e := store.Get(ctx, x.ID)
			if e != nil {
				return nil, e
			}
			if s == nil {
				out[i].Status = "expired_absent"
				continue
			}
			match, e := owned(x.ID, s)
			if e != nil || !match || !s.ExpiresAt.Equal(x.Expires) {
				return nil, errors.New("target session changed after ownership preflight")
			}
			if s.IsActive() {
				if e = revoker.RevokeSession(ctx, x.ID, "mock_cohort_cleanup", "system:10001"); e != nil {
					return nil, e
				}
			}
			s, e = store.Get(ctx, x.ID)
			if e != nil {
				return nil, e
			}
			if s == nil {
				out[i].Status = "expired_absent"
				continue
			}
			match, e = owned(x.ID, s)
			if e != nil || !match || s.IsActive() || !s.ExpiresAt.Equal(x.Expires) {
				return nil, errors.New("session revocation/identity/TTL readback failed")
			}
			out[i].Status = string(s.Status)
		}
	}
	return out, nil
}
func safeUser(s *sessiondomain.Session) string {
	if s == nil {
		return ""
	}
	return s.UserID.String()
}
func safeLogin(s *sessiondomain.Session) string {
	if s == nil {
		return ""
	}
	return s.LoginIdentityID.String()
}
