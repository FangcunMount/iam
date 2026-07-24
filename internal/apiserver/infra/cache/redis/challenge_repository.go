package redis

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	redisstore "github.com/FangcunMount/component-base/pkg/redis/store"
	"github.com/redis/go-redis/v9"

	cachegovernance "github.com/FangcunMount/iam/v2/internal/apiserver/application/cachegovernance"
	cachemodel "github.com/FangcunMount/iam/v2/internal/apiserver/cache"
	challengeDomain "github.com/FangcunMount/iam/v2/internal/apiserver/domain/authn/challenge"
)

type ChallengeRepository struct {
	client     *redis.Client
	challenges *redisstore.ValueStore[*challengeDomain.AuthChallenge]
}

var consumeChallengeIfSecretMatchesScript = redis.NewScript(`
local payload = redis.call("GET", KEYS[1])
if not payload then
	return 0
end
if not string.find(payload, ARGV[1], 1, true) then
	return 0
end
redis.call("DEL", KEYS[1])
redis.call("DEL", KEYS[2])
return 1
`)

var recordChallengeFailureIfCurrentScript = redis.NewScript(`
local payload = redis.call("GET", KEYS[1])
if not payload then
	return {0, 0}
end
if not string.find(payload, ARGV[1], 1, true) then
	return {0, 0}
end
local ttl = redis.call("PTTL", KEYS[1])
if ttl <= 0 then
	return {0, 0}
end
local attempts = redis.call("INCR", KEYS[2])
redis.call("PEXPIRE", KEYS[2], ttl)
if attempts >= tonumber(ARGV[2]) then
	redis.call("DEL", KEYS[1])
	redis.call("DEL", KEYS[2])
	return {1, 1}
end
return {1, 0}
`)

var _ challengeDomain.Repository = (*ChallengeRepository)(nil)

func NewChallengeRepository(client *redis.Client) *ChallengeRepository {
	return &ChallengeRepository{
		client:     client,
		challenges: newJSONStore[*challengeDomain.AuthChallenge](client),
	}
}

func (r *ChallengeRepository) Create(ctx context.Context, challenge *challengeDomain.AuthChallenge) error {
	if challenge == nil {
		return fmt.Errorf("challenge is required")
	}
	ttl := time.Until(challenge.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("challenge expires_at is in the past")
	}
	storeKey, err := newStoreKey(challengeRedisKey(challenge.ID))
	if err != nil {
		return err
	}
	return r.challenges.Set(ctx, storeKey, challenge, ttl)
}

func (r *ChallengeRepository) Get(ctx context.Context, id string) (*challengeDomain.AuthChallenge, error) {
	storeKey, err := newStoreKey(challengeRedisKey(id))
	if err != nil {
		return nil, err
	}
	challenge, found, err := r.challenges.Get(ctx, storeKey)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return challenge, nil
}

func (r *ChallengeRepository) ConsumeIfSecretMatches(ctx context.Context, id string, expectedHash []byte) (bool, error) {
	if len(expectedHash) == 0 {
		return false, nil
	}
	encoded := base64.StdEncoding.EncodeToString(expectedHash)
	needle := fmt.Sprintf(`"SecretHash":"%s"`, encoded)
	result, err := consumeChallengeIfSecretMatchesScript.Run(
		ctx,
		r.client,
		[]string{
			challengeRedisKey(id),
			challengeAttemptRedisKey(id, expectedHash),
		},
		needle,
	).Int()
	if err != nil {
		return false, fmt.Errorf("consume challenge if secret matches: %w", err)
	}
	return result == 1, nil
}

func (r *ChallengeRepository) RecordFailedAttemptIfCurrent(
	ctx context.Context,
	id string,
	currentSecretHash []byte,
	maxAttempts int,
) (bool, bool, error) {
	if len(currentSecretHash) == 0 || maxAttempts < 1 {
		return false, false, nil
	}
	encoded := base64.StdEncoding.EncodeToString(currentSecretHash)
	needle := fmt.Sprintf(`"SecretHash":"%s"`, encoded)
	result, err := recordChallengeFailureIfCurrentScript.Run(
		ctx,
		r.client,
		[]string{
			challengeRedisKey(id),
			challengeAttemptRedisKey(id, currentSecretHash),
		},
		needle,
		maxAttempts,
	).Slice()
	if err != nil {
		return false, false, fmt.Errorf("record challenge failure if current: %w", err)
	}
	if len(result) != 2 {
		return false, false, fmt.Errorf("record challenge failure if current: unexpected result")
	}
	current, ok := result[0].(int64)
	if !ok {
		return false, false, fmt.Errorf("record challenge failure if current: invalid current result")
	}
	exhausted, ok := result[1].(int64)
	if !ok {
		return false, false, fmt.Errorf("record challenge failure if current: invalid exhausted result")
	}
	return current == 1, exhausted == 1, nil
}

func challengeAttemptRedisKey(id string, secretHash []byte) string {
	version := base64.RawURLEncoding.EncodeToString(secretHash)
	return challengeRedisKey(id) + ":attempts:" + version
}

func (r *ChallengeRepository) Delete(ctx context.Context, id string) error {
	storeKey, err := newStoreKey(challengeRedisKey(id))
	if err != nil {
		return err
	}
	return r.challenges.Delete(ctx, storeKey)
}

func ChallengeRepositoryInspectors(repo *ChallengeRepository) []cachegovernance.FamilyInspector {
	if repo == nil {
		return nil
	}
	return []cachegovernance.FamilyInspector{
		newRedisFamilyInspector(cachemodel.FamilyAuthnChallenge, repo.client, "AuthN Challenge 采用 JSON 对象 + key 级 TTL 存储。"),
	}
}
