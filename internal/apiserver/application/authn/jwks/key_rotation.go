package jwks

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
)

// KeyRotationAppService 表示 JWKS 轮换应用服务。
type KeyRotationAppService struct {
	keyRotationSvc KeyRotatorPort
	keyPublisher   KeyPublisherPort
	logger         log.Logger
}

// NewKeyRotationAppService 创建 JWKS 轮换应用服务。
func NewKeyRotationAppService(
	keyRotationSvc KeyRotatorPort,
	logger log.Logger,
	publisher ...KeyPublisherPort,
) *KeyRotationAppService {
	service := &KeyRotationAppService{
		keyRotationSvc: keyRotationSvc,
		logger:         logger,
	}
	if len(publisher) > 0 {
		service.keyPublisher = publisher[0]
	}
	return service
}

// RotateKeyResponse 表示轮换密钥响应。
type RotateKeyResponse struct {
	NewKey  *RotatedKeyInfo // 新生成的密钥信息
	Rotated bool
}

// RotatedKeyInfo 表示轮换后的密钥信息。
type RotatedKeyInfo struct {
	Kid       string     // 密钥 ID
	Status    KeyStatus  // 密钥状态
	Algorithm string     // 签名算法
	NotBefore *time.Time // 生效时间
	NotAfter  *time.Time // 过期时间
	CreatedAt time.Time  // 创建时间
}

// RotateKey 执行密钥轮换
// 轮换流程由原子仓储转换 active/grace 状态；提交后刷新当前进程发布缓存。
func (s *KeyRotationAppService) RotateKey(ctx context.Context) (*RotateKeyResponse, error) {
	s.logger.Infow("Starting key rotation")

	newKey, err := s.keyRotationSvc.RotateKey(ctx)
	if err != nil {
		s.logger.Errorw("Key rotation failed", "error", err)
		return nil, err
	}
	s.refreshPublishCache(ctx)

	s.logger.Infow("Key rotation completed successfully",
		"newKid", newKey.Kid,
		"algorithm", newKey.JWK.Alg,
	)

	return &RotateKeyResponse{
		NewKey: &RotatedKeyInfo{
			Kid:       newKey.Kid,
			Status:    newKey.Status,
			Algorithm: newKey.JWK.Alg,
			NotBefore: newKey.NotBefore,
			NotAfter:  newKey.NotAfter,
			CreatedAt: newKey.CreatedAt,
		},
		Rotated: true,
	}, nil
}

// RotateIfDue checks and rotates in one repository transaction.
func (s *KeyRotationAppService) RotateIfDue(ctx context.Context) (*RotateKeyResponse, error) {
	active, rotated, err := s.keyRotationSvc.RotateIfDue(ctx)
	if err != nil {
		return nil, err
	}
	if active == nil {
		return &RotateKeyResponse{Rotated: false}, nil
	}
	if rotated {
		s.refreshPublishCache(ctx)
	}
	return &RotateKeyResponse{
		NewKey: &RotatedKeyInfo{
			Kid:       active.Kid,
			Status:    active.Status,
			Algorithm: active.JWK.Alg,
			NotBefore: active.NotBefore,
			NotAfter:  active.NotAfter,
			CreatedAt: active.CreatedAt,
		},
		Rotated: rotated,
	}, nil
}

func (s *KeyRotationAppService) refreshPublishCache(ctx context.Context) {
	if s.keyPublisher == nil {
		return
	}
	if err := s.keyPublisher.RefreshCache(ctx); err != nil {
		// The database transition is already committed. Keep the new signing
		// key active and make cache refresh observable for retry.
		s.logger.Warnw("jwks publish cache refresh failed after rotation", "error", err)
	}
}

// ShouldRotateResponse 是否需要轮换响应
type ShouldRotateResponse struct {
	ShouldRotate bool   // 是否需要轮换
	Reason       string // 原因说明
}

// ShouldRotate 判断是否需要轮换
// 根据 RotationPolicy 判断当前 Active 密钥是否已到轮换时间
func (s *KeyRotationAppService) ShouldRotate(ctx context.Context) (*ShouldRotateResponse, error) {
	s.logger.Debugw("Checking if key rotation is needed")

	shouldRotate, err := s.keyRotationSvc.ShouldRotate(ctx)
	if err != nil {
		s.logger.Errorw("Failed to check rotation need", "error", err)
		return nil, err
	}

	reason := "密钥未到轮换时间"
	if shouldRotate {
		reason = "密钥已到轮换时间"
	}

	s.logger.Debugw("Rotation check completed",
		"shouldRotate", shouldRotate,
		"reason", reason,
	)

	return &ShouldRotateResponse{
		ShouldRotate: shouldRotate,
		Reason:       reason,
	}, nil
}

// GetRotationPolicyResponse 获取轮换策略响应
type GetRotationPolicyResponse struct {
	Policy RotationPolicy // 轮换策略
}

// GetRotationPolicy 获取当前轮换策略
func (s *KeyRotationAppService) GetRotationPolicy(ctx context.Context) *GetRotationPolicyResponse {
	s.logger.Debugw("Getting rotation policy")

	policy := s.keyRotationSvc.GetRotationPolicy()

	s.logger.Debugw("Rotation policy retrieved",
		"rotationInterval", policy.RotationInterval,
		"gracePeriod", policy.GracePeriod,
		"maxKeysInJWKS", policy.MaxKeysInJWKS,
	)

	return &GetRotationPolicyResponse{
		Policy: policy,
	}
}

// UpdateRotationPolicyRequest 更新轮换策略请求
type UpdateRotationPolicyRequest struct {
	Policy RotationPolicy // 新的轮换策略
}

// UpdateRotationPolicy 更新轮换策略
func (s *KeyRotationAppService) UpdateRotationPolicy(ctx context.Context, req UpdateRotationPolicyRequest) error {
	s.logger.Infow("Updating rotation policy",
		"rotationInterval", req.Policy.RotationInterval,
		"gracePeriod", req.Policy.GracePeriod,
		"maxKeysInJWKS", req.Policy.MaxKeysInJWKS,
	)

	if err := s.keyRotationSvc.UpdateRotationPolicy(ctx, req.Policy); err != nil {
		s.logger.Errorw("Failed to update rotation policy", "error", err)
		return err
	}

	s.logger.Infow("Rotation policy updated successfully")
	return nil
}

// GetRotationStatusResponse 获取轮换状态响应
type GetRotationStatusResponse struct {
	Status RotationStatus // 轮换状态
}

// RotationStatus 轮换状态
type RotationStatus struct {
	LastRotation time.Time          // 上次轮换时间
	NextRotation time.Time          // 下次计划轮换时间
	ActiveKey    *RotationKeyInfo   // 当前激活的密钥
	GraceKeys    []*RotationKeyInfo // 宽限期密钥列表
	RetiredKeys  int                // 已退役密钥数量
	Policy       RotationPolicy     // 当前轮换策略
}

// RotationKeyInfo 轮换密钥信息
type RotationKeyInfo struct {
	Kid       string     // 密钥 ID
	Status    KeyStatus  // 密钥状态
	Algorithm string     // 签名算法
	NotBefore *time.Time // 生效时间
	NotAfter  *time.Time // 过期时间
	CreatedAt time.Time  // 创建时间
}
