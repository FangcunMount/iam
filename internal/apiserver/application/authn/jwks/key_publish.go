package jwks

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
)

// KeyPublishAppService JWKS 发布应用服务
// 负责构建和发布 /.well-known/jwks.json
type KeyPublishAppService struct {
	keyPublishSvc KeyPublisherPort
	logger        log.Logger
}

// NewKeyPublishAppService 创建 JWKS 发布应用服务
func NewKeyPublishAppService(
	keyPublishSvc KeyPublisherPort,
	logger log.Logger,
) *KeyPublishAppService {
	return &KeyPublishAppService{
		keyPublishSvc: keyPublishSvc,
		logger:        logger,
	}
}

// BuildJWKSResponse 构建 JWKS 响应
type BuildJWKSResponse struct {
	JWKS         []byte    // JWKS JSON 字节流
	ETag         string    // 实体标签（用于 HTTP 缓存）
	LastModified time.Time // 最后修改时间
}

// BuildJWKS 构建 JWKS JSON
// 用于 GET /.well-known/jwks.json 端点
func (s *KeyPublishAppService) BuildJWKS(ctx context.Context) (*BuildJWKSResponse, error) {
	s.logger.Debugw("Building JWKS")

	jwksJSON, tag, err := s.keyPublishSvc.BuildJWKS(ctx)
	if err != nil {
		s.logger.Errorw("Failed to build JWKS", "error", err)
		return nil, err
	}

	s.logger.Debugw("JWKS built successfully",
		"size", len(jwksJSON),
		"etag", tag.ETag,
		"lastModified", tag.LastModified,
	)

	return &BuildJWKSResponse{
		JWKS:         jwksJSON,
		ETag:         tag.ETag,
		LastModified: tag.LastModified,
	}, nil
}

// GetPublishableKeysResponse 获取可发布密钥响应
type GetPublishableKeysResponse struct {
	Keys []*PublishableKeyInfo // 可发布的密钥列表
}

// PublishableKeyInfo 可发布的密钥信息
type PublishableKeyInfo struct {
	Kid       string     // 密钥 ID
	Status    KeyStatus  // 密钥状态
	Algorithm string     // 签名算法
	NotBefore *time.Time // 生效时间
	NotAfter  *time.Time // 过期时间
	PublicJWK *PublicJWK // 公钥 JWK
}

// GetPublishableKeys 获取可发布的密钥列表
// 用于预览或调试，返回当前会被发布到 JWKS 的密钥
func (s *KeyPublishAppService) GetPublishableKeys(ctx context.Context) (*GetPublishableKeysResponse, error) {
	s.logger.Debugw("Getting publishable keys")

	keys, err := s.keyPublishSvc.GetPublishableKeys(ctx)
	if err != nil {
		s.logger.Errorw("Failed to get publishable keys", "error", err)
		return nil, err
	}

	s.logger.Debugw("Publishable keys retrieved", "count", len(keys))

	keyInfos := make([]*PublishableKeyInfo, len(keys))
	for i, key := range keys {
		keyInfos[i] = &PublishableKeyInfo{
			Kid:       key.Kid,
			Status:    key.Status,
			Algorithm: key.JWK.Alg,
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			PublicJWK: &key.JWK,
		}
	}

	return &GetPublishableKeysResponse{
		Keys: keyInfos,
	}, nil
}

// ValidateCacheTagRequest 验证缓存标签请求
type ValidateCacheTagRequest struct {
	ETag         string    // 客户端提供的 ETag
	LastModified time.Time // 客户端提供的 Last-Modified
}

// ValidateCacheTag 验证缓存标签
// 用于实现 HTTP 304 Not Modified 响应
// 返回 true 表示缓存有效（客户端缓存未过期）
func (s *KeyPublishAppService) ValidateCacheTag(ctx context.Context, req ValidateCacheTagRequest) (bool, error) {
	s.logger.Debugw("Validating cache tag",
		"clientETag", req.ETag,
		"clientLastModified", req.LastModified,
	)

	clientTag := CacheTag{
		ETag:         req.ETag,
		LastModified: req.LastModified,
	}

	isValid, err := s.keyPublishSvc.ValidateCacheTag(ctx, clientTag)
	if err != nil {
		s.logger.Errorw("Failed to validate cache tag", "error", err)
		return false, err
	}

	s.logger.Debugw("Cache tag validated", "isValid", isValid)

	return isValid, nil
}

// GetCurrentCacheTagResponse 获取当前缓存标签响应
type GetCurrentCacheTagResponse struct {
	ETag         string    // 当前 ETag
	LastModified time.Time // 当前最后修改时间
}

// GetCurrentCacheTag 获取当前缓存标签
// 用于生成 HTTP 响应头
func (s *KeyPublishAppService) GetCurrentCacheTag(ctx context.Context) (*GetCurrentCacheTagResponse, error) {
	s.logger.Debugw("Getting current cache tag")

	tag, err := s.keyPublishSvc.GetCurrentCacheTag(ctx)
	if err != nil {
		s.logger.Errorw("Failed to get current cache tag", "error", err)
		return nil, err
	}

	s.logger.Debugw("Current cache tag retrieved",
		"etag", tag.ETag,
		"lastModified", tag.LastModified,
	)

	return &GetCurrentCacheTagResponse{
		ETag:         tag.ETag,
		LastModified: tag.LastModified,
	}, nil
}

// RefreshCache 刷新缓存
// 用于强制更新缓存（密钥轮换后）
func (s *KeyPublishAppService) RefreshCache(ctx context.Context) error {
	s.logger.Infow("Refreshing JWKS cache")

	if err := s.keyPublishSvc.RefreshCache(ctx); err != nil {
		s.logger.Errorw("Failed to refresh cache", "error", err)
		return err
	}

	s.logger.Infow("JWKS cache refreshed successfully")
	return nil
}
