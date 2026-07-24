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
	// RecordFailedAttemptIfCurrent 仅对调用方读取到的同一版本挑战记录失败。
	// 达到 maxAttempts 时原子删除该挑战。
	RecordFailedAttemptIfCurrent(
		ctx context.Context,
		id string,
		currentSecretHash []byte,
		maxAttempts int,
	) (current bool, exhausted bool, err error)
	// Delete 删除挑战
	Delete(ctx context.Context, id string) error
}
