package challenge

import "context"

// Repository 挑战仓储接口
type Repository interface {
	// Create 创建挑战
	Create(ctx context.Context, challenge *AuthChallenge) error
	// Get 获取挑战
	Get(ctx context.Context, id string) (*AuthChallenge, error)
	// ConsumeIfSecretMatches 仅在挑战仍存在且密钥哈希未变化时原子消费挑战。
	ConsumeIfSecretMatches(ctx context.Context, id string, expectedHash []byte) (bool, error)
	// Delete 删除挑战
	Delete(ctx context.Context, id string) error
}
