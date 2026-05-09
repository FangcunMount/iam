package challenge

import "time"

type AuthChallenge struct {
	ID         string
	Type       ChallengeType
	Scene      string
	Target     string
	SecretHash []byte
	ExpiresAt  time.Time
	Attempts   int
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

func (c *AuthChallenge) IsExpired(now time.Time) bool {
	return c == nil || !now.Before(c.ExpiresAt)
}

func (c *AuthChallenge) IsConsumed() bool {
	return c != nil && c.ConsumedAt != nil
}

func (c *AuthChallenge) ConsumeAt(now time.Time) {
	c.ConsumedAt = &now
}
