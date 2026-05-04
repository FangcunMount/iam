package redis

import (
	"fmt"

	redislease "github.com/FangcunMount/component-base/pkg/redis/lease"
	"github.com/redis/go-redis/v9"
)

func newLeaseService(client *redis.Client) *redislease.Service {
	return redislease.NewService(client)
}

func newLeaseKey(key string) (redislease.LeaseKey, error) {
	leaseKey, err := redislease.NewLeaseKey(key)
	if err != nil {
		return "", fmt.Errorf("invalid lease key %q: %w", key, err)
	}
	return leaseKey, nil
}
