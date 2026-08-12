package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	perrors "github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	redisstore "github.com/FangcunMount/component-base/pkg/redis/store"
	cachegovernance "github.com/FangcunMount/iam/v3/internal/apiserver/application/cachegovernance"
	cachemodel "github.com/FangcunMount/iam/v3/internal/apiserver/cache"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	"github.com/FangcunMount/iam/v3/internal/pkg/meta"
	"github.com/redis/go-redis/v9"
)

const (
	sessionTransactionMaxAttempts = 5
	sessionTransactionBackoff     = 5 * time.Millisecond
)

// SessionStore 基于 Redis 承载认证会话与用户/登录身份索引。
type SessionStore struct {
	client       *redis.Client
	sessionStore *redisstore.ValueStore[*sessiondomain.Session]
}

var _ sessiondomain.Store = (*SessionStore)(nil)

// NewSessionStore 创建 Redis 会话存储。
func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{
		client:       client,
		sessionStore: newJSONStore[*sessiondomain.Session](client),
	}
}

// Save 保存或覆盖会话主对象，并维护用户/登录身份索引。
func (s *SessionStore) Save(ctx context.Context, sess *sessiondomain.Session) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	if sess == nil {
		return fmt.Errorf("session is nil")
	}
	ttl := sess.RemainingTTL()
	if ttl <= 0 {
		return fmt.Errorf("session ttl must be positive")
	}
	payload, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("encode session payload: %w", err)
	}
	_, err = s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, sessionRedisKey(sess.SessionID), payload, ttl)
		s.addIndexesToPipeline(ctx, pipe, sess)
		return nil
	})
	if err != nil {
		return fmt.Errorf("save session and indexes: %w", err)
	}
	return nil
}

// Get 按 sid 读取会话。
func (s *SessionStore) Get(ctx context.Context, sessionID string) (*sessiondomain.Session, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	key, err := newStoreKey(sessionRedisKey(sessionID))
	if err != nil {
		return nil, err
	}
	sess, found, err := s.sessionStore.Get(ctx, key)
	if err != nil || !found {
		return sess, err
	}
	if sess != nil && sess.IsExpired() && sess.Status == sessiondomain.StatusActive {
		sess.Status = sessiondomain.StatusExpired
	}
	return sess, nil
}

// Revoke 撤销指定会话，并移除 user/login identity 索引。
func (s *SessionStore) Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	key := sessionRedisKey(sessionID)
	return s.withSessionTransactionRetry(ctx, key, func(tx *redis.Tx) error {
		sess, err := loadSessionForTransaction(ctx, tx, key)
		if err != nil || sess == nil {
			return err
		}
		sess.Revoke(reason, revokedBy)
		payload, err := json.Marshal(sess)
		if err != nil {
			return fmt.Errorf("encode revoked session: %w", err)
		}
		ttl := sess.RemainingTTL()
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if ttl <= 0 {
				pipe.Del(ctx, key)
			} else {
				pipe.Set(ctx, key, payload, ttl)
			}
			s.removeIndexesFromPipeline(ctx, pipe, sess)
			return nil
		})
		return err
	})
}

// Extend 延长会话过期时间，并同步索引 score。
func (s *SessionStore) Extend(ctx context.Context, sessionID string, expiresAt time.Time) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	key := sessionRedisKey(sessionID)
	return s.withSessionTransactionRetry(ctx, key, func(tx *redis.Tx) error {
		sess, err := loadSessionForTransaction(ctx, tx, key)
		if err != nil || sess == nil {
			return err
		}
		if !sess.IsActive() {
			return perrors.WithCode(code.ErrSessionInactive, "session has been revoked or expired")
		}
		sess.Extend(expiresAt)
		ttl := sess.RemainingTTL()
		if ttl <= 0 {
			return perrors.WithCode(code.ErrSessionInactive, "session extension must remain active")
		}
		payload, err := json.Marshal(sess)
		if err != nil {
			return fmt.Errorf("encode extended session: %w", err)
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, key, payload, ttl)
			s.addIndexesToPipeline(ctx, pipe, sess)
			return nil
		})
		return err
	})
}

// RevokeByUser 撤销指定用户下的全部活跃会话。
func (s *SessionStore) RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error {
	return s.revokeByIndex(ctx, userSessionIndexRedisKey(userID.String()), reason, revokedBy)
}

// RevokeByLoginIdentity 撤销指定登录身份下的全部活跃会话。
func (s *SessionStore) RevokeByLoginIdentity(ctx context.Context, loginIdentityID meta.ID, reason string, revokedBy string) error {
	return s.revokeByIndex(ctx, loginIdentitySessionIndexRedisKey(loginIdentityID.String()), reason, revokedBy)
}

func (s *SessionStore) revokeByIndex(ctx context.Context, indexKey string, reason string, revokedBy string) error {
	if err := s.removeExpiredIndexMembers(ctx, indexKey); err != nil {
		return err
	}
	sessionIDs, err := s.client.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("list indexed sessions: %w", err)
	}
	for _, sessionID := range sessionIDs {
		sess, getErr := s.Get(ctx, sessionID)
		if getErr != nil {
			return getErr
		}
		if sess == nil {
			if err := s.client.ZRem(ctx, indexKey, sessionID).Err(); err != nil {
				return fmt.Errorf("remove stale session index member: %w", err)
			}
			continue
		}
		if err := s.Revoke(ctx, sessionID, reason, revokedBy); err != nil {
			return err
		}
	}
	return nil
}

func (s *SessionStore) addIndexesToPipeline(ctx context.Context, pipe redis.Pipeliner, sess *sessiondomain.Session) {
	userIndexKey := userSessionIndexRedisKey(sess.UserID.String())
	loginIdentityIndexKey := loginIdentitySessionIndexRedisKey(sess.LoginIdentityID.String())
	score := float64(sess.ExpiresAt.Unix())
	pipe.ZAdd(ctx, userIndexKey, redis.Z{Score: score, Member: sess.SessionID})
	pipe.ZAdd(ctx, loginIdentityIndexKey, redis.Z{Score: score, Member: sess.SessionID})
}

func (s *SessionStore) removeIndexesFromPipeline(ctx context.Context, pipe redis.Pipeliner, sess *sessiondomain.Session) {
	userIndexKey := userSessionIndexRedisKey(sess.UserID.String())
	loginIdentityIndexKey := loginIdentitySessionIndexRedisKey(sess.LoginIdentityID.String())
	pipe.ZRem(ctx, userIndexKey, sess.SessionID)
	pipe.ZRem(ctx, loginIdentityIndexKey, sess.SessionID)
}

func loadSessionForTransaction(ctx context.Context, tx *redis.Tx, key string) (*sessiondomain.Session, error) {
	payload, err := tx.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load session transaction payload: %w", err)
	}
	var sess sessiondomain.Session
	if err := json.Unmarshal(payload, &sess); err != nil {
		return nil, fmt.Errorf("decode session transaction payload: %w", err)
	}
	return &sess, nil
}

func (s *SessionStore) withSessionTransactionRetry(
	ctx context.Context,
	key string,
	update func(*redis.Tx) error,
) error {
	var lastErr error
	for attempt := 1; attempt <= sessionTransactionMaxAttempts; attempt++ {
		err := s.client.Watch(ctx, update, key)
		if err == nil {
			return nil
		}
		if !errors.Is(err, redis.TxFailedErr) {
			return err
		}
		lastErr = err
		if attempt == sessionTransactionMaxAttempts {
			redisError(ctx, "session optimistic transaction retries exhausted",
				log.Int("attempts", sessionTransactionMaxAttempts))
			break
		}
		timer := time.NewTimer(time.Duration(attempt) * sessionTransactionBackoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("session optimistic transaction conflict: %w", lastErr)
}

func (s *SessionStore) removeExpiredIndexMembers(ctx context.Context, indexKey string) error {
	nowScore := strconv.FormatInt(time.Now().Unix(), 10)
	return s.client.ZRemRangeByScore(ctx, indexKey, "-inf", nowScore).Err()
}

// FamilyInspectors 返回 session 相关缓存族的状态读取器。
func (s *SessionStore) FamilyInspectors() []cachegovernance.FamilyInspector {
	if s == nil {
		return nil
	}
	return []cachegovernance.FamilyInspector{
		newRedisFamilyInspector(cachemodel.FamilyAuthnSession, s.client, "会话主对象使用 Redis String(JSON) 存储。"),
		newRedisFamilyInspector(cachemodel.FamilyAuthnUserSessionIndex, s.client, "用户维度会话索引使用 Redis ZSet。"),
		newRedisFamilyInspector(cachemodel.FamilyAuthnLoginIdentitySessionIndex, s.client, "登录身份维度会话索引使用 Redis ZSet。"),
	}
}
