package shared

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
	policyDomain "github.com/FangcunMount/iam/internal/apiserver/domain/authz/policy"
)

type cacheInvalidator interface {
	InvalidateCache()
}

// ReloadRuntimePolicy 将运行时 Casbin 缓存刷新到最新数据库事实。
func ReloadRuntimePolicy(ctx context.Context, adapter policyDomain.CasbinAdapter, operation string) {
	if adapter == nil {
		return
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if invalidator, ok := adapter.(cacheInvalidator); ok {
			invalidator.InvalidateCache()
		}
		if err := adapter.LoadPolicy(ctx); err == nil {
			return
		} else {
			lastErr = err
			log.Errorw("failed to reload authz runtime policy",
				"operation", operation,
				"attempt", attempt,
				"error", err,
			)
		}
		if attempt < 3 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	log.Errorw("authz runtime policy remains degraded after reload retries",
		"operation", operation,
		"error", lastErr,
	)
}
