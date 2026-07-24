package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
)

// RefreshTokenPurgeResult is deliberately limited to aggregate counts.
type RefreshTokenPurgeResult struct {
	Scanned int64 `json:"scanned"`
	Matched int64 `json:"matched"`
	Deleted int64 `json:"deleted"`
}

// PurgeRefreshTokens scans only the refresh-token keyspace and optionally
// removes matching keys in bounded batches. It never returns key names.
func PurgeRefreshTokens(
	ctx context.Context,
	client goredis.UniversalClient,
	batchSize int64,
	apply bool,
) (RefreshTokenPurgeResult, error) {
	var result RefreshTokenPurgeResult
	if client == nil {
		return result, fmt.Errorf("redis client is required")
	}
	if batchSize <= 0 {
		return result, fmt.Errorf("batch size must be positive")
	}

	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, refreshTokenKeyspace.Prefix("*"), batchSize).Result()
		if err != nil {
			return result, fmt.Errorf("scan refresh token keyspace: %w", err)
		}
		count := int64(len(keys))
		result.Scanned += count
		result.Matched += count

		if apply && len(keys) > 0 {
			deleted, err := client.Unlink(ctx, keys...).Result()
			if err != nil {
				return result, fmt.Errorf("unlink refresh token batch: %w", err)
			}
			result.Deleted += deleted
		}
		cursor = next
		if cursor == 0 {
			return result, nil
		}
	}
}
