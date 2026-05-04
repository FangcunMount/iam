package redis

import (
	"context"
	"fmt"
	"time"

	redisstore "github.com/FangcunMount/component-base/pkg/redis/store"
	"github.com/redis/go-redis/v9"

	"github.com/FangcunMount/component-base/pkg/log"
	tokenapp "github.com/FangcunMount/iam/v2/internal/apiserver/application/authn/token"
	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	cachemodel "github.com/FangcunMount/iam/v2/internal/apiserver/cache"
	"github.com/FangcunMount/iam/v2/internal/pkg/meta"
)

// RedisStore Redis 令牌存储实现
type RedisStore struct {
	client                    *redis.Client
	refreshTokens             *redisstore.ValueStore[refreshTokenData]
	revokedAccessTokenMarkers *redisstore.ValueStore[string]
}

// NewRedisStore 创建 Redis 令牌存储
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{
		client:                    client,
		refreshTokens:             newJSONStore[refreshTokenData](client),
		revokedAccessTokenMarkers: newStringStore(client),
	}
}

// FamilyInspectors 返回当前适配器暴露的缓存族状态读取器。
func (s *RedisStore) FamilyInspectors() []cachegovernance.FamilyInspector {
	return []cachegovernance.FamilyInspector{
		newRedisFamilyInspector(cachemodel.FamilyAuthnRefreshToken, s.client, "刷新令牌采用 JSON String 存储。"),
		newRedisFamilyInspector(cachemodel.FamilyAuthnRevokedAccessToken, s.client, "已撤销访问令牌采用 marker String 存储。"),
	}
}

// refreshTokenData 刷新令牌存储数据结构
type refreshTokenData struct {
	TokenID       string            `json:"token_id"`
	SessionID     string            `json:"session_id"`
	UserID        uint64            `json:"user_id"`
	AccountID     uint64            `json:"account_id"`
	TenantID      uint64            `json:"tenant_id"`
	Amr           []string          `json:"amr,omitempty"`
	SessionClaims map[string]string `json:"session_claims,omitempty"`
	ExpiresAt     time.Time         `json:"expires_at"`
}

// SaveRefreshToken 保存刷新令牌
func (s *RedisStore) SaveRefreshToken(ctx context.Context, token *tokenapp.Token) error {
	if token == nil {
		return fmt.Errorf("token is nil")
	}

	data := refreshTokenData{
		TokenID:       token.ID,
		SessionID:     token.SessionID,
		UserID:        token.UserID.Uint64(),
		AccountID:     token.AccountID.Uint64(),
		TenantID:      token.TenantID.Uint64(),
		Amr:           token.AMR,
		SessionClaims: token.SessionClaims,
		ExpiresAt:     token.ExpiresAt,
	}

	// 保存到 Redis，key 格式: refresh_token:{token_value}
	key := refreshTokenRedisKey(token.Value)
	storeKey, err := newStoreKey(key)
	if err != nil {
		return err
	}
	ttl := token.RemainingDuration()
	if ttl <= 0 {
		redisWarn(ctx, "attempted to save expired refresh token", log.String("token_id", token.ID))
		return fmt.Errorf("token already expired")
	}

	if err := s.refreshTokens.Set(ctx, storeKey, data, ttl); err != nil {
		return fmt.Errorf("failed to save refresh token to redis: %w", err)
	}

	redisInfo(ctx, "refresh token cached",
		log.String("token_id", token.ID),
		log.String("key", key),
		log.Duration("ttl", ttl),
	)
	return nil
}

// GetRefreshToken 获取刷新令牌
func (s *RedisStore) GetRefreshToken(ctx context.Context, tokenValue string) (*tokenapp.Token, error) {
	key := refreshTokenRedisKey(tokenValue)
	storeKey, err := newStoreKey(key)
	if err != nil {
		return nil, err
	}

	data, found, err := s.refreshTokens.Get(ctx, storeKey)
	if err != nil {
		redisError(ctx, "failed to load refresh token", log.String("error", err.Error()))
		return nil, fmt.Errorf("failed to get refresh token from redis: %w", err)
	}
	if !found {
		return nil, nil
	}

	// 构造 Token 对象
	ttl := time.Until(data.ExpiresAt)
	userID := meta.FromUint64(data.UserID)
	accountID := meta.FromUint64(data.AccountID)
	tenantID := meta.FromUint64(data.TenantID)
	token := tokenapp.NewRefreshToken(
		data.TokenID,
		tokenValue,
		data.SessionID,
		userID,
		accountID,
		tenantID,
		data.Amr,
		data.SessionClaims,
		ttl,
	)

	// Redis Hook 已经记录了 GET 命令成功，这里不需要再记录 cache hit
	return token, nil
}

// DeleteRefreshToken 删除刷新令牌
func (s *RedisStore) DeleteRefreshToken(ctx context.Context, tokenValue string) error {
	key := refreshTokenRedisKey(tokenValue)
	storeKey, err := newStoreKey(key)
	if err != nil {
		return err
	}

	if err := s.refreshTokens.Delete(ctx, storeKey); err != nil {
		return fmt.Errorf("failed to delete refresh token from redis: %w", err)
	}

	redisInfo(ctx, "refresh token deleted", log.String("key", key))
	return nil
}

// MarkAccessTokenRevoked 标记访问令牌已撤销。
func (s *RedisStore) MarkAccessTokenRevoked(ctx context.Context, tokenID string, expiry time.Duration) error {
	key := revokedAccessTokenRedisKey(tokenID)
	storeKey, err := newStoreKey(key)
	if err != nil {
		return err
	}

	// 设置撤销标记，TTL 为令牌剩余有效期。
	if err := s.revokedAccessTokenMarkers.Set(ctx, storeKey, "1", expiry); err != nil {
		return fmt.Errorf("failed to mark access token revoked: %w", err)
	}

	redisInfo(ctx, "access token marked revoked", log.String("token_id", tokenID), log.Duration("ttl", expiry))
	return nil
}

// IsAccessTokenRevoked 检查访问令牌是否已撤销。
func (s *RedisStore) IsAccessTokenRevoked(ctx context.Context, tokenID string) (bool, error) {
	key := revokedAccessTokenRedisKey(tokenID)
	storeKey, err := newStoreKey(key)
	if err != nil {
		return false, err
	}

	// 检查撤销标记是否存在。
	exists, err := s.revokedAccessTokenMarkers.Exists(ctx, storeKey)
	if err != nil {
		return false, fmt.Errorf("failed to check revoked access token marker: %w", err)
	}

	return exists, nil
}
