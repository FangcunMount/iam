package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	redisstore "github.com/FangcunMount/component-base/pkg/redis/store"
	cachegovernance "github.com/FangcunMount/iam/internal/apiserver/application/cachegovernance"
	sessiondomain "github.com/FangcunMount/iam/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/internal/pkg/meta"
	"github.com/redis/go-redis/v9"
)

// SessionStore 基于 Redis 承载认证会话与用户/账号索引。
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

// Save 保存或覆盖会话主对象，并维护用户/账号索引。
func (s *SessionStore) Save(ctx context.Context, sess *sessiondomain.Session) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	if sess == nil {
		return fmt.Errorf("session is nil")
	}
	key, err := newStoreKey(sessionRedisKey(sess.SessionID))
	if err != nil {
		return err
	}
	ttl := sess.RemainingTTL()
	if ttl <= 0 {
		return fmt.Errorf("session ttl must be positive")
	}
	if err := s.sessionStore.Set(ctx, key, sess, ttl); err != nil {
		return fmt.Errorf("save session payload: %w", err)
	}
	if err := s.addIndexes(ctx, sess); err != nil {
		return fmt.Errorf("save session indexes: %w", err)
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

// Revoke 撤销指定会话，并移除 user/account 索引。
func (s *SessionStore) Revoke(ctx context.Context, sessionID string, reason string, revokedBy string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	sess, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess == nil {
		return nil
	}
	sess.Revoke(reason, revokedBy)
	if err := s.removeIndexes(ctx, sess); err != nil {
		return fmt.Errorf("remove session indexes: %w", err)
	}
	ttl := sess.RemainingTTL()
	key, err := newStoreKey(sessionRedisKey(sessionID))
	if err != nil {
		return err
	}
	if ttl <= 0 {
		return s.sessionStore.Delete(ctx, key)
	}
	return s.sessionStore.Set(ctx, key, sess, ttl)
}

// Extend 延长会话过期时间，并同步索引 score。
func (s *SessionStore) Extend(ctx context.Context, sessionID string, expiresAt time.Time) error {
	sess, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess == nil {
		return nil
	}
	sess.Extend(expiresAt)
	return s.Save(ctx, sess)
}

// RevokeByUser 撤销指定用户下的全部活跃会话。
func (s *SessionStore) RevokeByUser(ctx context.Context, userID meta.ID, reason string, revokedBy string) error {
	return s.revokeByIndex(ctx, userSessionIndexRedisKey(userID.String()), reason, revokedBy)
}

// RevokeByAccount 撤销指定账号下的全部活跃会话。
func (s *SessionStore) RevokeByAccount(ctx context.Context, accountID meta.ID, reason string, revokedBy string) error {
	return s.revokeByIndex(ctx, accountSessionIndexRedisKey(accountID.String()), reason, revokedBy)
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
		if err := s.Revoke(ctx, sessionID, reason, revokedBy); err != nil {
			return err
		}
	}
	return nil
}

func (s *SessionStore) addIndexes(ctx context.Context, sess *sessiondomain.Session) error {
	userIndexKey := userSessionIndexRedisKey(sess.UserID.String())
	accountIndexKey := accountSessionIndexRedisKey(sess.AccountID.String())
	score := float64(sess.ExpiresAt.Unix())
	pipe := s.client.TxPipeline()
	pipe.ZAdd(ctx, userIndexKey, redis.Z{Score: score, Member: sess.SessionID})
	pipe.ZAdd(ctx, accountIndexKey, redis.Z{Score: score, Member: sess.SessionID})
	_, err := pipe.Exec(ctx)
	return err
}

func (s *SessionStore) removeIndexes(ctx context.Context, sess *sessiondomain.Session) error {
	userIndexKey := userSessionIndexRedisKey(sess.UserID.String())
	accountIndexKey := accountSessionIndexRedisKey(sess.AccountID.String())
	pipe := s.client.TxPipeline()
	pipe.ZRem(ctx, userIndexKey, sess.SessionID)
	pipe.ZRem(ctx, accountIndexKey, sess.SessionID)
	_, err := pipe.Exec(ctx)
	return err
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
		newRedisFamilyInspector(cachegovernance.FamilyAuthnSession, s.client, "会话主对象使用 Redis String(JSON) 存储。"),
		newRedisFamilyInspector(cachegovernance.FamilyAuthnUserSessionIndex, s.client, "用户维度会话索引使用 Redis ZSet。"),
		newRedisFamilyInspector(cachegovernance.FamilyAuthnAccountSessionIndex, s.client, "账号维度会话索引使用 Redis ZSet。"),
	}
}
