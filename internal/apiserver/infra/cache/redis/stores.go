package redis

import (
	"fmt"

	redisstore "github.com/FangcunMount/component-base/pkg/redis/store"
	"github.com/redis/go-redis/v9"
)

func newJSONStore[T any](client *redis.Client) *redisstore.ValueStore[T] {
	return redisstore.NewValueStore[T](client, redisstore.JSONCodec[T]{})
}

func newStringStore(client *redis.Client) *redisstore.ValueStore[string] {
	return redisstore.NewValueStore[string](client, redisstore.StringCodec{})
}

func newStoreKey(key string) (redisstore.StoreKey, error) {
	storeKey, err := redisstore.NewStoreKey(key)
	if err != nil {
		return "", fmt.Errorf("invalid redis key %q: %w", key, err)
	}
	return storeKey, nil
}
