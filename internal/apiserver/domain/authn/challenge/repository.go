package challenge

import "context"

// Repository 挑战仓储接口
type Repository interface {
	// Create 创建挑战
	Create(ctx context.Context, challenge *AuthChallenge) error
	// Get 获取挑战
	Get(ctx context.Context, id string) (*AuthChallenge, error)
	// Consume 消费挑战
	Consume(ctx context.Context, id string) error
	// Delete 删除挑战
	Delete(ctx context.Context, id string) error
}
