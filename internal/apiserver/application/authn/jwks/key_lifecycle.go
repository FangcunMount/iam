package jwks

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/errors"
	"github.com/FangcunMount/component-base/pkg/log"
	"github.com/FangcunMount/iam/v3/internal/pkg/code"
	pkgauth "github.com/FangcunMount/iam/v3/pkg/auth"
)

const (
	operationAdminCreate = "admin_create"
	operationAutoRotate  = "auto_rotate"
	operationEnterGrace  = "enter_grace"
	operationRetire      = "retire"
	operationForceRetire = "force_retire"
	operationCleanup     = "cleanup"
)

// KeyLifecycleAppService is the single application boundary for JWKS mutations.
type KeyLifecycleAppService struct {
	lifecycle KeyLifecyclePort
	publisher KeyPublisherPort
	observer  LifecycleObserver
	logger    log.Logger
}

func NewKeyLifecycleAppService(
	lifecycle KeyLifecyclePort,
	publisher KeyPublisherPort,
	observer LifecycleObserver,
	logger log.Logger,
) *KeyLifecycleAppService {
	return &KeyLifecycleAppService{
		lifecycle: lifecycle,
		publisher: publisher,
		observer:  observer,
		logger:    logger,
	}
}

type CreateKeyRequest struct {
	Algorithm string
	NotBefore *time.Time
	NotAfter  *time.Time
}

type CreateKeyResponse struct {
	Kid       string
	Status    string
	Algorithm string
	NotBefore *time.Time
	NotAfter  *time.Time
	PublicJWK *PublicJWK
	CreatedAt time.Time
}

func (s *KeyLifecycleAppService) CreateAndActivate(ctx context.Context, req CreateKeyRequest) (*CreateKeyResponse, error) {
	if req.Algorithm != pkgauth.TokenProfileAlgorithm {
		return nil, errors.WithCode(code.ErrInvalidArgument, "algorithm must be %s", pkgauth.TokenProfileAlgorithm)
	}
	key, changed, err := s.lifecycle.CreateAndActivate(ctx, req.Algorithm, req.NotBefore, req.NotAfter)
	if err != nil {
		s.record(operationAdminCreate, "failed")
		s.logger.Errorw("jwks lifecycle operation failed", "operation", operationAdminCreate, "result", "failed", "automatic", false)
		return nil, err
	}
	s.record(operationAdminCreate, mutationResult(changed))
	if changed {
		s.refreshPublishCache(ctx, operationAdminCreate, key.Kid, false)
	}
	s.logger.Infow("jwks lifecycle operation completed",
		"operation", operationAdminCreate,
		"result", mutationResult(changed),
		"kid", key.Kid,
		"automatic", false,
	)
	return createKeyResponse(key), nil
}

type RotateKeyResponse struct {
	NewKey  *RotatedKeyInfo
	Rotated bool
}

type RotatedKeyInfo struct {
	Kid       string
	Status    string
	Algorithm string
	NotBefore *time.Time
	NotAfter  *time.Time
	CreatedAt time.Time
}

func (s *KeyLifecycleAppService) RotateIfDue(ctx context.Context) (*RotateKeyResponse, error) {
	key, changed, err := s.lifecycle.RotateIfDue(ctx)
	if err != nil {
		s.record(operationAutoRotate, "failed")
		s.logger.Errorw("jwks lifecycle operation failed", "operation", operationAutoRotate, "result", "failed", "automatic", true)
		return nil, err
	}
	s.record(operationAutoRotate, mutationResult(changed))
	if key == nil {
		s.logger.Debugw("jwks lifecycle operation completed",
			"operation", operationAutoRotate,
			"result", "noop",
			"automatic", true,
		)
		return &RotateKeyResponse{Rotated: false}, nil
	}
	if changed {
		s.refreshPublishCache(ctx, operationAutoRotate, key.Kid, true)
	}
	s.logger.Infow("jwks lifecycle operation completed",
		"operation", operationAutoRotate,
		"result", mutationResult(changed),
		"kid", key.Kid,
		"automatic", true,
	)
	return &RotateKeyResponse{
		NewKey: &RotatedKeyInfo{
			Kid:       key.Kid,
			Status:    key.Status,
			Algorithm: key.JWK.Alg,
			NotBefore: key.NotBefore,
			NotAfter:  key.NotAfter,
			CreatedAt: key.CreatedAt,
		},
		Rotated: changed,
	}, nil
}

func (s *KeyLifecycleAppService) RetireKey(ctx context.Context, kid string) error {
	return s.mutateKey(ctx, operationRetire, kid, s.lifecycle.RetireKey)
}

func (s *KeyLifecycleAppService) ForceRetireKey(ctx context.Context, kid string) error {
	return s.mutateKey(ctx, operationForceRetire, kid, s.lifecycle.ForceRetireKey)
}

func (s *KeyLifecycleAppService) EnterGracePeriod(ctx context.Context, kid string) error {
	return s.mutateKey(ctx, operationEnterGrace, kid, s.lifecycle.EnterGracePeriod)
}

type CleanupExpiredKeysResponse struct {
	DeletedCount int
}

func (s *KeyLifecycleAppService) CleanupExpiredKeys(ctx context.Context) (*CleanupExpiredKeysResponse, error) {
	count, err := s.lifecycle.CleanupExpiredKeys(ctx)
	if err != nil {
		s.record(operationCleanup, "failed")
		s.logger.Errorw("jwks lifecycle operation failed", "operation", operationCleanup, "result", "failed", "automatic", false)
		return nil, err
	}
	changed := count > 0
	s.record(operationCleanup, mutationResult(changed))
	if changed {
		s.refreshPublishCache(ctx, operationCleanup, "", false)
	}
	s.logger.Infow("jwks lifecycle operation completed",
		"operation", operationCleanup,
		"result", mutationResult(changed),
		"automatic", false,
	)
	return &CleanupExpiredKeysResponse{DeletedCount: count}, nil
}

func (s *KeyLifecycleAppService) mutateKey(
	ctx context.Context,
	operation, kid string,
	mutate func(context.Context, string) error,
) error {
	if err := mutate(ctx, kid); err != nil {
		s.record(operation, "failed")
		s.logger.Errorw("jwks lifecycle operation failed", "operation", operation, "result", "failed", "kid", kid, "automatic", false)
		return err
	}
	s.record(operation, "success")
	s.refreshPublishCache(ctx, operation, kid, false)
	s.logger.Infow("jwks lifecycle operation completed", "operation", operation, "result", "success", "kid", kid, "automatic", false)
	return nil
}

func (s *KeyLifecycleAppService) refreshPublishCache(ctx context.Context, operation, kid string, automatic bool) {
	if s.publisher == nil {
		return
	}
	if err := s.publisher.RefreshCache(ctx); err != nil {
		if s.observer != nil {
			s.observer.RecordPostCommitFailure("cache_refresh")
		}
		s.logger.Warnw("jwks publish cache refresh failed after committed lifecycle operation",
			"operation", operation,
			"result", "failed",
			"kid", kid,
			"automatic", automatic,
		)
	}
}

func (s *KeyLifecycleAppService) record(operation, result string) {
	if s.observer != nil {
		s.observer.RecordOperation(operation, result)
	}
}

func mutationResult(changed bool) string {
	if changed {
		return "success"
	}
	return "noop"
}

func createKeyResponse(key *ManagedKey) *CreateKeyResponse {
	if key == nil {
		return nil
	}
	return &CreateKeyResponse{
		Kid:       key.Kid,
		Status:    key.Status,
		Algorithm: key.JWK.Alg,
		NotBefore: key.NotBefore,
		NotAfter:  key.NotAfter,
		PublicJWK: &key.JWK,
		CreatedAt: key.CreatedAt,
	}
}
