package jwks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/iam/internal/pkg/code"
)

// KeySetBuilder JWKS 构建服务
// 实现 Publisher 接口
type KeySetBuilder struct {
	keyRepo Repository
	// 缓存最后构建的 JWKS
	lastJWKS      *JWKS
	lastCacheTag  CacheTag
	lastBuildTime time.Time
}

// NewKeySetBuilder 创建 JWKS 构建器
func NewKeySetBuilder(keyRepo Repository) *KeySetBuilder {
	return &KeySetBuilder{
		keyRepo: keyRepo,
	}
}

// Ensure KeySetBuilder implements KeySetPublishService
var _ Publisher = (*KeySetBuilder)(nil)

// BuildJWKS 构建 JWKS JSON
// 查询所有可发布的密钥（Active + Grace 状态且未过期）
func (s *KeySetBuilder) BuildJWKS(ctx context.Context) ([]byte, CacheTag, error) {
	// 获取可发布的密钥
	keys, err := s.keyRepo.FindPublishable(ctx)
	if err != nil {
		return nil, CacheTag{}, errors.WithCode(code.ErrDatabase, "failed to find publishable keys: %v", err)
	}

	if len(keys) == 0 {
		// 没有可发布的密钥，返回空 JWKS
		emptyJWKS := JWKS{Keys: []PublicJWK{}}
		jwksJSON, err := json.Marshal(emptyJWKS)
		if err != nil {
			return nil, CacheTag{}, errors.WithCode(code.ErrEncodingJSON, "failed to marshal empty JWKS: %v", err)
		}

		// 生成缓存标签
		tag := s.generateCacheTag(jwksJSON)
		return jwksJSON, tag, nil
	}

	// 提取公钥并构建 JWKS
	publicKeys := make([]PublicJWK, 0, len(keys))
	for _, key := range keys {
		if key.ShouldPublish() {
			publicKeys = append(publicKeys, key.JWK)
		}
	}

	// 按 kid 排序，确保输出稳定
	sort.Slice(publicKeys, func(i, j int) bool {
		return publicKeys[i].Kid < publicKeys[j].Kid
	})

	// 构建 JWKS 对象
	jwksObj := JWKS{Keys: publicKeys}

	// 序列化为 JSON
	jwksJSON, err := json.Marshal(jwksObj)
	if err != nil {
		return nil, CacheTag{}, errors.WithCode(code.ErrEncodingJSON, "failed to marshal JWKS: %v", err)
	}

	// 生成缓存标签
	tag := s.generateCacheTag(jwksJSON)

	// 更新缓存
	s.lastJWKS = &jwksObj
	s.lastCacheTag = tag
	s.lastBuildTime = time.Now()

	return jwksJSON, tag, nil
}

// GetPublishableKeys 获取可发布的密钥列表
func (s *KeySetBuilder) GetPublishableKeys(ctx context.Context) ([]*Key, error) {
	keys, err := s.keyRepo.FindPublishable(ctx)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find publishable keys: %v", err)
	}

	// 过滤出应该发布的密钥
	publishable := make([]*Key, 0, len(keys))
	for _, key := range keys {
		if key.ShouldPublish() {
			publishable = append(publishable, key)
		}
	}

	return publishable, nil
}

// ValidateCacheTag 验证缓存标签
// 返回 true 表示缓存有效（未变更），可以返回 304 Not Modified
func (s *KeySetBuilder) ValidateCacheTag(ctx context.Context, clientTag CacheTag) (bool, error) {
	// 获取当前缓存标签
	currentTag, err := s.GetCurrentCacheTag(ctx)
	if err != nil {
		return false, err
	}

	// 比较 ETag
	if clientTag.ETag != "" && currentTag.ETag != "" {
		return clientTag.ETag == currentTag.ETag, nil
	}

	// 比较 Last-Modified（精确到秒）
	if !clientTag.LastModified.IsZero() && !currentTag.LastModified.IsZero() {
		// 截断到秒级别，因为 HTTP 头只支持秒级精度
		clientTime := clientTag.LastModified.Truncate(time.Second)
		currentTime := currentTag.LastModified.Truncate(time.Second)
		return !clientTime.Before(currentTime), nil
	}

	// 无法验证，认为缓存无效
	return false, nil
}

// GetCurrentCacheTag 获取当前缓存标签
func (s *KeySetBuilder) GetCurrentCacheTag(ctx context.Context) (CacheTag, error) {
	// 如果有缓存且未过期（1分钟内），直接返回
	if s.lastCacheTag.ETag != "" && time.Since(s.lastBuildTime) < time.Minute {
		return s.lastCacheTag, nil
	}

	// 重新构建 JWKS 获取最新标签
	_, tag, err := s.BuildJWKS(ctx)
	if err != nil {
		return CacheTag{}, err
	}

	return tag, nil
}

// RefreshCache 刷新缓存
func (s *KeySetBuilder) RefreshCache(ctx context.Context) error {
	// 重新构建 JWKS
	_, _, err := s.BuildJWKS(ctx)
	return err
}

// SnapshotStatus 返回当前进程内 JWKS 快照的只读状态。
func (s *KeySetBuilder) SnapshotStatus() SnapshotStatus {
	if s == nil {
		return SnapshotStatus{}
	}

	status := SnapshotStatus{
		Cached:   s.lastJWKS != nil,
		CacheTag: s.lastCacheTag,
	}
	if s.lastJWKS != nil {
		status.KeyCount = len(s.lastJWKS.Keys)
	}
	if !s.lastBuildTime.IsZero() {
		lastBuildTime := s.lastBuildTime
		status.LastBuildTime = &lastBuildTime
	}
	return status
}

// generateCacheTag 生成缓存标签
func (s *KeySetBuilder) generateCacheTag(content []byte) CacheTag {
	// 生成 ETag（使用 SHA-256 哈希的前 16 字节）
	hash := sha256.Sum256(content)
	etag := `"` + hex.EncodeToString(hash[:16]) + `"`

	// 使用当前时间作为 Last-Modified
	lastModified := time.Now().UTC()

	return CacheTag{
		ETag:         etag,
		LastModified: lastModified,
	}
}

// GetJWKSStats 获取 JWKS 统计信息（辅助方法）
func (s *KeySetBuilder) GetJWKSStats(ctx context.Context) (*JWKSStats, error) {
	keys, err := s.keyRepo.FindPublishable(ctx)
	if err != nil {
		return nil, errors.WithCode(code.ErrDatabase, "failed to find publishable keys: %v", err)
	}

	stats := &JWKSStats{
		TotalKeys:  len(keys),
		ActiveKeys: 0,
		GraceKeys:  0,
	}

	for _, key := range keys {
		if key.IsActive() {
			stats.ActiveKeys++
		} else if key.IsGrace() {
			stats.GraceKeys++
		}
	}

	// 最后构建时间
	if !s.lastBuildTime.IsZero() {
		stats.LastBuildTime = &s.lastBuildTime
	}

	return stats, nil
}

// JWKSStats JWKS 统计信息
type JWKSStats struct {
	TotalKeys     int        // 可发布的密钥总数
	ActiveKeys    int        // Active 状态的密钥数
	GraceKeys     int        // Grace 状态的密钥数
	LastBuildTime *time.Time // 最后构建时间
}

// SnapshotStatus 表示当前进程内 JWKS 发布快照的状态。
type SnapshotStatus struct {
	Cached        bool       // 是否已经构建过进程内快照
	KeyCount      int        // 快照中的公钥数量
	CacheTag      CacheTag   // 最近一次构建生成的缓存标签
	LastBuildTime *time.Time // 最近一次构建时间
}

// GetCacheControl 获取缓存控制策略（辅助方法）
// 返回适合 HTTP Cache-Control 头的值
func (s *KeySetBuilder) GetCacheControl() string {
	// JWKS 应该被缓存，但不应缓存太久
	// 推荐：public（可被共享缓存）, max-age=3600（1小时）, must-revalidate（过期后必须重新验证）
	return "public, max-age=3600, must-revalidate"
}

// ValidateJWKS 验证 JWKS 完整性（辅助方法）
func (s *KeySetBuilder) ValidateJWKS(ctx context.Context) error {
	keys, err := s.keyRepo.FindPublishable(ctx)
	if err != nil {
		return errors.WithCode(code.ErrDatabase, "failed to find publishable keys: %v", err)
	}

	if len(keys) == 0 {
		return errors.WithCode(code.ErrNoActiveKey, "no publishable keys available")
	}

	// 验证每个密钥
	for _, key := range keys {
		if err := key.Validate(); err != nil {
			return errors.WithCode(code.ErrInvalidJWK, "invalid key %s: %v", key.Kid, err)
		}
	}

	return nil
}
