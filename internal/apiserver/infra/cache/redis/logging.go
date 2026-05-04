package redis

import (
	"context"

	"github.com/FangcunMount/component-base/pkg/log"
)

func redisFields(ctx context.Context, fields []log.Field) []log.Field {
	if ctx == nil {
		return fields
	}
	return append(fields, log.TraceFields(ctx)...)
}

func redisInfo(ctx context.Context, msg string, fields ...log.Field) {
	log.Redis(msg, redisFields(ctx, fields)...)
}

func redisWarn(ctx context.Context, msg string, fields ...log.Field) {
	log.RedisWarn(msg, redisFields(ctx, fields)...)
}

func redisError(ctx context.Context, msg string, fields ...log.Field) {
	log.RedisError(msg, redisFields(ctx, fields)...)
}
