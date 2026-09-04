package signingkey

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
)

// KeyManagementAppService 签名密钥管理查询应用服务。
type KeyManagementAppService struct {
	keyReader KeyReaderPort
	logger    log.Logger
}

// NewKeyManagementAppService 创建密钥管理应用服务
func NewKeyManagementAppService(
	keyReader KeyReaderPort,
	logger log.Logger,
) *KeyManagementAppService {
	return &KeyManagementAppService{
		keyReader: keyReader,
		logger:    logger,
	}
}

// GetActiveKeyResponse 获取激活密钥响应
// 用于获取当前激活的密钥
type GetActiveKeyResponse struct {
	Kid       string     // 密钥 ID
	Status    string     // 密钥状态
	Algorithm string     // 签名算法
	NotBefore *time.Time // 生效时间
	NotAfter  *time.Time // 过期时间
	PublicJWK *PublicJWK // 公钥 JWK
}

// GetActiveKey 获取当前激活的密钥
func (s *KeyManagementAppService) GetActiveKey(ctx context.Context) (*GetActiveKeyResponse, error) {
	s.logger.Debugw("Getting active key")

	key, err := s.keyReader.GetActiveKey(ctx)
	if err != nil {
		s.logger.Errorw("Failed to get active key", "error", err)
		return nil, err
	}

	s.logger.Debugw("Active key retrieved",
		"kid", key.Kid,
		"algorithm", key.Algorithm,
	)

	return &GetActiveKeyResponse{
		Kid:       key.Kid,
		Status:    key.Status,
		Algorithm: key.Algorithm,
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
		PublicJWK: &key.JWK,
	}, nil
}

// GetKeyByKidResponse 根据 kid 获取密钥响应
type GetKeyByKidResponse struct {
	Kid       string     // 密钥 ID
	Status    string     // 密钥状态
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

	key, err := s.keyReader.GetKeyByKid(ctx, kid)
	if err != nil {
		s.logger.Errorw("Failed to get key by kid", "kid", kid, "error", err)
		return nil, err
	}

	s.logger.Debugw("Key retrieved", "kid", kid, "status", key.Status)

	return &GetKeyByKidResponse{
		Kid:       key.Kid,
		Status:    key.Status,
		Algorithm: key.Algorithm,
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
		PublicJWK: &key.JWK,
		CreatedAt: key.CreatedAt,
		UpdatedAt: key.UpdatedAt,
	}, nil
}

// ListKeysRequest 列出密钥请求
type ListKeysRequest struct {
	Status string // 状态过滤（可选）
	Limit  int    // 每页数量
	Offset int    // 偏移量
}

// ListKeysResponse 列出密钥响应
type ListKeysResponse struct {
	Keys  []*KeyInfo // 密钥列表
	Total int64      // 总数
}

// KeyInfo 密钥信息
type KeyInfo struct {
	Kid       string     // 密钥 ID
	Status    string     // 密钥状态
	Algorithm string     // 签名算法
	NotBefore *time.Time // 生效时间
	NotAfter  *time.Time // 过期时间
	PublicJWK *PublicJWK // 公钥 JWK
	CreatedAt time.Time  // 创建时间
	UpdatedAt time.Time  // 更新时间
}

// ListKeys 列出密钥（分页）
func (s *KeyManagementAppService) ListKeys(ctx context.Context, req ListKeysRequest) (*ListKeysResponse, error) {
	if !validStatusFilter(req.Status) {
		return nil, errors.WithCode(code.ErrInvalidArgument, "invalid status filter")
	}
	s.logger.Debugw("Listing keys",
		"status", req.Status,
		"limit", req.Limit,
		"offset", req.Offset,
	)

	keys, total, err := s.keyReader.ListKeys(ctx, req.Status, req.Limit, req.Offset)
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
			Algorithm: key.Algorithm,
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

func validStatusFilter(status string) bool {
	switch status {
	case "", "active", "grace", "retired":
		return true
	default:
		return false
	}
}
