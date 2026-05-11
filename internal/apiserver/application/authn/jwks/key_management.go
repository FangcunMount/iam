package jwks

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
)

// KeyManagementAppService JWKS 管理应用服务
type KeyManagementAppService struct {
	keyMgmtSvc KeyManagerPort
	logger     log.Logger
}

// NewKeyManagementAppService 创建密钥管理应用服务
func NewKeyManagementAppService(
	keyMgmtSvc KeyManagerPort,
	logger log.Logger,
) *KeyManagementAppService {
	return &KeyManagementAppService{
		keyMgmtSvc: keyMgmtSvc,
		logger:     logger,
	}
}

// CreateKeyRequest 创建密钥请求
type CreateKeyRequest struct {
	Algorithm string     // 签名算法（RS256/RS384/RS512）
	NotBefore *time.Time // 生效时间（可选）
	NotAfter  *time.Time // 过期时间（可选）
}

// CreateKeyResponse 创建密钥响应
type CreateKeyResponse struct {
	Kid       string     // 密钥 ID
	Status    KeyStatus  // 密钥状态
	Algorithm string     // 签名算法
	NotBefore *time.Time // 生效时间
	NotAfter  *time.Time // 过期时间
	PublicJWK *PublicJWK // 公钥 JWK
	CreatedAt time.Time  // 创建时间
}

// CreateKey 创建新密钥
func (s *KeyManagementAppService) CreateKey(ctx context.Context, req CreateKeyRequest) (*CreateKeyResponse, error) {
	s.logger.Infow("Creating new key",
		"algorithm", req.Algorithm,
		"notBefore", req.NotBefore,
		"notAfter", req.NotAfter,
	)

	key, err := s.keyMgmtSvc.CreateKey(ctx, req.Algorithm, req.NotBefore, req.NotAfter)
	if err != nil {
		s.logger.Errorw("Failed to create key", "error", err)
		return nil, err
	}

	s.logger.Infow("Key created successfully",
		"kid", key.Kid,
		"status", key.Status,
		"algorithm", key.JWK.Alg,
	)

	return &CreateKeyResponse{
		Kid:       key.Kid,
		Status:    key.Status,
		Algorithm: key.JWK.Alg,
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
		PublicJWK: &key.JWK,
		CreatedAt: key.CreatedAt,
	}, nil
}

// GetActiveKeyResponse 获取激活密钥响应
// 用于获取当前激活的密钥
type GetActiveKeyResponse struct {
	Kid       string     // 密钥 ID
	Status    KeyStatus  // 密钥状态
	Algorithm string     // 签名算法
	NotBefore *time.Time // 生效时间
	NotAfter  *time.Time // 过期时间
	PublicJWK *PublicJWK // 公钥 JWK
}

// GetActiveKey 获取当前激活的密钥
func (s *KeyManagementAppService) GetActiveKey(ctx context.Context) (*GetActiveKeyResponse, error) {
	s.logger.Debugw("Getting active key")

	key, err := s.keyMgmtSvc.GetActiveKey(ctx)
	if err != nil {
		s.logger.Errorw("Failed to get active key", "error", err)
		return nil, err
	}

	s.logger.Debugw("Active key retrieved",
		"kid", key.Kid,
		"algorithm", key.JWK.Alg,
	)

	return &GetActiveKeyResponse{
		Kid:       key.Kid,
		Status:    key.Status,
		Algorithm: key.JWK.Alg,
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
		PublicJWK: &key.JWK,
	}, nil
}

// GetKeyByKidResponse 根据 kid 获取密钥响应
type GetKeyByKidResponse struct {
	Kid       string     // 密钥 ID
	Status    KeyStatus  // 密钥状态
	Algorithm string     // 签名算法
	NotBefore *time.Time // 生效时间
	NotAfter  *time.Time // 过期时间
	PublicJWK *PublicJWK // 公钥 JWK
	CreatedAt time.Time  // 创建时间
	UpdatedAt time.Time  // 更新时间
}

// GetKeyByKid 根据 kid 获取密钥
func (s *KeyManagementAppService) GetKeyByKid(ctx context.Context, kid string) (*GetKeyByKidResponse, error) {
	s.logger.Debugw("Getting key by kid", "kid", kid)

	key, err := s.keyMgmtSvc.GetKeyByKid(ctx, kid)
	if err != nil {
		s.logger.Errorw("Failed to get key by kid", "kid", kid, "error", err)
		return nil, err
	}

	s.logger.Debugw("Key retrieved", "kid", kid, "status", key.Status)

	return &GetKeyByKidResponse{
		Kid:       key.Kid,
		Status:    key.Status,
		Algorithm: key.JWK.Alg,
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
		PublicJWK: &key.JWK,
		CreatedAt: key.CreatedAt,
		UpdatedAt: key.UpdatedAt,
	}, nil
}

// RetireKey 退役密钥（Grace → Retired）
// 只能对 Grace 状态的密钥执行
// kid: 密钥 ID
func (s *KeyManagementAppService) RetireKey(ctx context.Context, kid string) error {
	s.logger.Infow("Retiring key", "kid", kid)

	if err := s.keyMgmtSvc.RetireKey(ctx, kid); err != nil {
		s.logger.Errorw("Failed to retire key", "kid", kid, "error", err)
		return err
	}

	s.logger.Infow("Key retired successfully", "kid", kid)
	return nil
}

// ForceRetireKey 强制退役密钥（任何状态 → Retired）
// 只能对 Grace 状态的密钥执行
// kid: 密钥 ID
func (s *KeyManagementAppService) ForceRetireKey(ctx context.Context, kid string) error {
	s.logger.Warnw("Force retiring key", "kid", kid)

	if err := s.keyMgmtSvc.ForceRetireKey(ctx, kid); err != nil {
		s.logger.Errorw("Failed to force retire key", "kid", kid, "error", err)
		return err
	}

	s.logger.Warnw("Key force retired successfully", "kid", kid)
	return nil
}

// EnterGracePeriod 进入宽限期（Active → Grace）
// 只能对 Active 状态的密钥执行
// kid: 密钥 ID
func (s *KeyManagementAppService) EnterGracePeriod(ctx context.Context, kid string) error {
	s.logger.Infow("Moving key to grace period", "kid", kid)

	if err := s.keyMgmtSvc.EnterGracePeriod(ctx, kid); err != nil {
		s.logger.Errorw("Failed to move key to grace period", "kid", kid, "error", err)
		return err
	}

	s.logger.Infow("Key moved to grace period successfully", "kid", kid)
	return nil
}

// CleanupExpiredKeysResponse 清理过期密钥响应
// 用于清理过期密钥
type CleanupExpiredKeysResponse struct {
	DeletedCount int // 清理的密钥数量
}

// CleanupExpiredKeys 清理过期密钥
// 删除 NotAfter < now 且 Status = Retired 的密钥
func (s *KeyManagementAppService) CleanupExpiredKeys(ctx context.Context) (*CleanupExpiredKeysResponse, error) {
	s.logger.Infow("Cleaning up expired keys")

	count, err := s.keyMgmtSvc.CleanupExpiredKeys(ctx)
	if err != nil {
		s.logger.Errorw("Failed to cleanup expired keys", "error", err)
		return nil, err
	}

	s.logger.Infow("Expired keys cleaned up", "deletedCount", count)

	return &CleanupExpiredKeysResponse{
		DeletedCount: count,
	}, nil
}

// ListKeysRequest 列出密钥请求
type ListKeysRequest struct {
	Status KeyStatus // 状态过滤（可选）
	Limit  int       // 每页数量
	Offset int       // 偏移量
}

// ListKeysResponse 列出密钥响应
type ListKeysResponse struct {
	Keys  []*KeyInfo // 密钥列表
	Total int64      // 总数
}

// KeyInfo 密钥信息
type KeyInfo struct {
	Kid       string     // 密钥 ID
	Status    KeyStatus  // 密钥状态
	Algorithm string     // 签名算法
	NotBefore *time.Time // 生效时间
	NotAfter  *time.Time // 过期时间
	PublicJWK *PublicJWK // 公钥 JWK
	CreatedAt time.Time  // 创建时间
	UpdatedAt time.Time  // 更新时间
}

// ListKeys 列出密钥（分页）
func (s *KeyManagementAppService) ListKeys(ctx context.Context, req ListKeysRequest) (*ListKeysResponse, error) {
	s.logger.Debugw("Listing keys",
		"status", req.Status,
		"limit", req.Limit,
		"offset", req.Offset,
	)

	keys, total, err := s.keyMgmtSvc.ListKeys(ctx, req.Status, req.Limit, req.Offset)
	if err != nil {
		s.logger.Errorw("Failed to list keys", "error", err)
		return nil, err
	}

	s.logger.Debugw("Keys listed",
		"count", len(keys),
		"total", total,
	)

	keyInfos := make([]*KeyInfo, len(keys))
	for i, key := range keys {
		keyInfos[i] = &KeyInfo{
			Kid:       key.Kid,
			Status:    key.Status,
			Algorithm: key.JWK.Alg,
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			PublicJWK: &key.JWK,
			CreatedAt: key.CreatedAt,
			UpdatedAt: key.UpdatedAt,
		}
	}

	return &ListKeysResponse{
		Keys:  keyInfos,
		Total: total,
	}, nil
}
