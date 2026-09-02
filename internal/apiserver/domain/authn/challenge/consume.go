package challenge

import (
	"context"
	"crypto/subtle"
	"time"
)

// Usability 描述挑战当前是否可用于校验。
type Usability int

const (
	UsabilityOK Usability = iota
	UsabilityNotFound
	UsabilityWrongType
	UsabilityWrongScene
	UsabilityConsumed
	UsabilityExpired
)

// AssessUsability 评估挑战是否满足类型、场景与生命周期约束。
func AssessUsability(ch *AuthChallenge, now time.Time, typ ChallengeType, scene string) Usability {
	if ch == nil {
		return UsabilityNotFound
	}
	if ch.Type != typ {
		return UsabilityWrongType
	}
	if ch.Scene != scene {
		return UsabilityWrongScene
	}
	if ch.IsExpired(now) {
		return UsabilityExpired
	}
	return UsabilityOK
}

// normalizeVerificationTime 标准化验证时间。
func normalizeVerificationTime(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now()
	}
	return now
}

// secretHashMatches 比较两个密钥哈希是否匹配。
func secretHashMatches(stored, expected []byte) bool {
	return subtle.ConstantTimeCompare(stored, expected) == 1
}

// consumeOnce 消费一次挑战。
func consumeOnce(ctx context.Context, repo Repository, challengeID string, expectedHash []byte) (VerificationResult, error) {
	// 消费挑战
	consumed, err := repo.ConsumeIfSecretMatches(ctx, challengeID, expectedHash)
	if err != nil {
		return VerificationResult{Outcome: VerificationInfrastructureError}, err
	}
	// 如果挑战未被消费，则返回验证失败
	if !consumed {
		return VerificationResult{Outcome: VerificationRejected}, nil
	}
	// 返回验证成功
	return VerificationResult{Outcome: VerificationSuccess}, nil
}

// recordFailedVerification 记录失败验证。
func recordFailedVerification(
	ctx context.Context,
	repo Repository,
	challengeID string,
	currentSecretHash []byte,
	maxAttempts int,
) (VerificationResult, error) {
	current, exhausted, err := repo.RecordFailedAttemptIfCurrent(ctx, challengeID, currentSecretHash, maxAttempts)
	if err != nil {
		return VerificationResult{Outcome: VerificationInfrastructureError}, err
	}
	if current && exhausted {
		return VerificationResult{Outcome: VerificationExhausted}, nil
	}
	return VerificationResult{Outcome: VerificationRejected}, nil
}
