package shared

import (
	"context"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
)

type RuntimePolicyReloader interface {
	LoadPolicy(ctx context.Context) error
}

type cacheInvalidator interface {
	InvalidateCache()
}

// ReloadRuntimePolicy 将运行时 Casbin 缓存刷新到最新数据库事实。
func ReloadRuntimePolicy(ctx context.Context, adapter RuntimePolicyReloader, operation string) {
	if adapter == nil {
		return
	}

	started := time.Now()
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if invalidator, ok := adapter.(cacheInvalidator); ok {
			invalidator.InvalidateCache()
		}
		if err := adapter.LoadPolicy(ctx); err == nil {
			log.InfoContext(ctx, "authz runtime policy reload completed",
				log.String("operation", operation),
				log.String("result", "success"),
				log.Int("attempt", attempt),
				log.Int64("duration_ms", time.Since(started).Milliseconds()),
			)
			return
		} else {
			lastErr = err
			log.ErrorContext(ctx, "failed to reload authz runtime policy",
				log.String("operation", operation),
				log.String("result", "failed"),
				log.Int("attempt", attempt),
				log.Int64("duration_ms", time.Since(started).Milliseconds()),
				log.Err(err),
			)
		}
		if attempt < 3 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	log.ErrorContext(ctx, "authz runtime policy remains degraded after reload retries",
		log.String("operation", operation),
		log.String("result", "degraded"),
		log.Int64("duration_ms", time.Since(started).Milliseconds()),
		log.Err(lastErr),
	)
}
