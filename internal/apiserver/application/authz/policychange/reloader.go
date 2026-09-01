package policychange

import (
	"context"
	"fmt"
	"time"

	"github.com/FangcunMount/component-base/pkg/log"
)

type RuntimePolicyReloader interface {
	LoadPolicy(ctx context.Context) error
}

// ReloadRuntimePolicy 从最新数据库事实重建并原子替换授权快照。
func ReloadRuntimePolicy(ctx context.Context, adapter RuntimePolicyReloader, operation string) {
	_ = ReloadRuntimePolicyWithError(ctx, adapter, operation)
}

// ReloadRuntimePolicyWithError 从最新数据库事实重建并原子替换授权快照，
// 并将最终失败返回给需要重试语义的调用方（例如策略版本事件消费者）。
func ReloadRuntimePolicyWithError(ctx context.Context, adapter RuntimePolicyReloader, operation string) error {
	if adapter == nil {
		return nil
	}

	started := time.Now()
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := adapter.LoadPolicy(ctx); err == nil {
			log.InfoContext(ctx, "authz runtime policy reload completed",
				log.String("operation", operation),
				log.String("result", "success"),
				log.Int("attempt", attempt),
				log.Int64("duration_ms", time.Since(started).Milliseconds()),
			)
			return nil
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
			select {
			case <-ctx.Done():
				return fmt.Errorf("reload authz runtime policy after %s: %w", operation, ctx.Err())
			case <-time.After(100 * time.Millisecond):
			}
		}
	}

	log.ErrorContext(ctx, "authz runtime policy remains degraded after reload retries",
		log.String("operation", operation),
		log.String("result", "degraded"),
		log.Int64("duration_ms", time.Since(started).Milliseconds()),
		log.Err(lastErr),
	)
	return fmt.Errorf("reload authz runtime policy after %s: %w", operation, lastErr)
}
