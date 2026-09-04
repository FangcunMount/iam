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
	cachegovernance "github.com/FangcunMount/iam/v3/internal/apiserver/application/cachegovernance"
	cachemodel "github.com/FangcunMount/iam/v3/internal/apiserver/cache"
	"github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/authentication"
	sessiondomain "github.com/FangcunMount/iam/v3/internal/apiserver/domain/authn/session"
	"github.com/FangcunMount/iam/v3/internal/pkg/authnclaims"
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
	client *redis.Client
}

var _ sessiondomain.Store = (*SessionStore)(nil)

// NewSessionStore 创建 Redis 会话存储。
func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{
		client: client,
	}
}

const currentSessionSchemaVersion = 2

type sessionAuthenticationContextData struct {
	Method          string    `json:"method,omitempty"`
	Realm           string    `json:"realm,omitempty"`
	AMR             []string  `json:"amr,omitempty"`
	AuthenticatedAt time.Time `json:"authenticated_at,omitempty"`
}

type sessionTokenContextData struct {
	TenantDomain string            `json:"tenant_domain,omitempty"`
	OrgID        uint64            `json:"org_id,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

// sessionData 是 Redis wire model。PascalCase 字段用于兼容既有 JSON；
// schema_version/auth_context/token_context 是新写入的强类型扩展。
type sessionData struct {
	SchemaVersion   int                               `json:"schema_version,omitempty"`
	SessionID       string                            `json:"SessionID"`
	UserID          uint64                            `json:"UserID"`
	LoginIdentityID uint64                            `json:"LoginIdentityID"`
	TenantID        uint64                            `json:"TenantID"`
	AuthContext     *sessionAuthenticationContextData `json:"auth_context,omitempty"`
	TokenContext    *sessionTokenContextData          `json:"token_context,omitempty"`
	Status          sessiondomain.Status              `json:"Status"`
	CreatedAt       time.Time                         `json:"CreatedAt"`
	ExpiresAt       time.Time                         `json:"ExpiresAt"`
	RevokedAt       *time.Time                        `json:"RevokedAt,omitempty"`
	RevokeReason    string                            `json:"RevokeReason,omitempty"`
	RevokedBy       string                            `json:"RevokedBy,omitempty"`

	// 仅用于读取 v1 历史 JSON；v2 写入不再填充。
	AuthMethod      string            `json:"AuthMethod,omitempty"`
	Realm           string            `json:"Realm,omitempty"`
	AMR             []string          `json:"AMR,omitempty"`
	AuthenticatedAt time.Time         `json:"AuthenticatedAt,omitempty"`
	SessionClaims   map[string]string `json:"SessionClaims,omitempty"`
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
	payload, err := encodeSessionPayload(sess)
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
	payload, err := s.client.Get(ctx, sessionRedisKey(sessionID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sess, err := decodeSessionPayload(payload)
	if err != nil {
		return nil, err
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
		payload, err := encodeSessionPayload(sess)
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
		payload, err := encodeSessionPayload(sess)
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
	return decodeSessionPayload(payload)
}

func encodeSessionPayload(sess *sessiondomain.Session) ([]byte, error) {
	if sess == nil {
		return nil, fmt.Errorf("session is nil")
	}
	authContext := sess.AuthContext.Clone()
	tokenContext := sess.TokenContext.Clone()
	data := sessionData{
		SchemaVersion: currentSessionSchemaVersion,
		SessionID:     sess.SessionID, UserID: sess.UserID.Uint64(), LoginIdentityID: sess.LoginIdentityID.Uint64(), TenantID: sess.TenantID.Uint64(),
		AuthContext: &sessionAuthenticationContextData{
			Method: string(authContext.Method), Realm: authContext.Realm,
			AMR: authContext.AMRStrings(), AuthenticatedAt: authContext.AuthenticatedAt,
		},
		TokenContext: &sessionTokenContextData{
			TenantDomain: tokenContext.TenantDomain, OrgID: tokenContext.OrgID.Uint64(), Attributes: tokenContext.Attributes,
		},
		Status: sess.Status, CreatedAt: sess.CreatedAt, ExpiresAt: sess.ExpiresAt,
		RevokedAt: sess.RevokedAt, RevokeReason: sess.RevokeReason, RevokedBy: sess.RevokedBy,
	}
	return json.Marshal(data)
}

func decodeSessionPayload(payload []byte) (*sessiondomain.Session, error) {
	var data sessionData
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("decode session payload: %w", err)
	}
	authContext := authentication.RestoreAuthenticationContext(
		authentication.Method(data.AuthMethod), data.Realm, amrValues(data.AMR), data.AuthenticatedAt,
	)
	if data.AuthContext != nil {
		authContext = authentication.RestoreAuthenticationContext(
			authentication.Method(data.AuthContext.Method), data.AuthContext.Realm,
			amrValues(data.AuthContext.AMR), data.AuthContext.AuthenticatedAt,
		)
	}
	tokenContext := authentication.TokenContext{}
	if data.TokenContext != nil {
		tokenContext = authentication.TokenContext{
			TenantDomain: data.TokenContext.TenantDomain,
			OrgID:        meta.FromUint64(data.TokenContext.OrgID),
			Attributes:   cloneStringValues(data.TokenContext.Attributes),
		}
	} else if len(data.SessionClaims) > 0 {
		legacy := authnclaims.DecodeSnapshot(data.SessionClaims)
		if domain, ok := legacy["tenant_domain"].(string); ok {
			tokenContext.TenantDomain = domain
		}
		if raw, ok := legacy["org_id"].(string); ok {
			if id, err := meta.ParseID(raw); err == nil {
				tokenContext.OrgID = id
			}
		}
		tokenContext.Attributes = authnclaims.EncodeJWTAttributes(legacy)
	}
	sess := &sessiondomain.Session{
		SessionID: data.SessionID, UserID: meta.FromUint64(data.UserID), LoginIdentityID: meta.FromUint64(data.LoginIdentityID), TenantID: meta.FromUint64(data.TenantID),
		AuthContext: authContext, TokenContext: tokenContext,
		Status: data.Status, CreatedAt: data.CreatedAt, ExpiresAt: data.ExpiresAt,
		RevokedAt: data.RevokedAt, RevokeReason: data.RevokeReason, RevokedBy: data.RevokedBy,
	}
	return sess, nil
}

func amrValues(values []string) []authentication.AMR {
	out := make([]authentication.AMR, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, authentication.AMR(value))
		}
	}
	return out
}

func cloneStringValues(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
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
