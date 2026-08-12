package idp

import (
	"fmt"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	infraRedis "github.com/FangcunMount/iam/v3/internal/apiserver/infra/cache/redis"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/crypto"
	infraMysql "github.com/FangcunMount/iam/v3/internal/apiserver/infra/mysql/wechatapp"
	"github.com/FangcunMount/iam/v3/internal/apiserver/infra/wechatapi"
)

func (m *IDPModule) initializeInfrastructure(
	db *gorm.DB,
	redisClient *redis.Client,
	encryptionKey []byte,
) error {
	m.wechatAppRepo = infraMysql.NewWechatAppRepository(db)
	m.accessTokenCache = infraRedis.NewAccessTokenCache(redisClient)

	secretVault, err := crypto.NewSecretVault(encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to create secret vault: %w", err)
	}
	m.secretVault = secretVault

	wechatSDKCache := infraRedis.NewWechatSDKCache(redisClient)
	m.wechatSDKCache = wechatSDKCache
	m.wechatAuthProvider = wechatapi.NewAuthProvider(wechatSDKCache)
	m.wechatTokenProvider = wechatapi.NewTokenProvider(wechatSDKCache)

	return nil
}
