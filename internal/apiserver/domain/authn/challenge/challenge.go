package challenge

import "time"

// AuthChallenge 认证挑战
type AuthChallenge struct {
	ID   string        // 挑战ID
	Type ChallengeType // 挑战类型

	// 挑战上下文
	Scene      string            // 场景
	Target     string            // 目标
	SecretHash []byte            // 密钥哈希
	Payload    map[string]string // 附加上下文（如 OAuth state 的 app_id、redirect_uri）

	// 挑战状态
	Attempts   int        // 尝试次数
	ConsumedAt *time.Time // 消费时间
	ExpiresAt  time.Time  // 过期时间
	CreatedAt  time.Time  // 创建时间
}

// IsExpired 验证挑战是否过期
func (c *AuthChallenge) IsExpired(now time.Time) bool {
	return c == nil || !now.Before(c.ExpiresAt)
}

// IsConsumed 验证挑战是否已消费
func (c *AuthChallenge) IsConsumed() bool {
	return c != nil && c.ConsumedAt != nil
}

// ConsumeAt 消费挑战
func (c *AuthChallenge) ConsumeAt(now time.Time) {
	c.ConsumedAt = &now
}
