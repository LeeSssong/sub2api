package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const openAITurnStateKeyPrefix = "openai:turn_state_reuse:"

var openAITurnStatePutScript = redis.NewScript(`
local current = redis.call("HGET", KEYS[1], "issued_at")
if current and tonumber(current) >= tonumber(ARGV[2]) then
  return 0
end
redis.call("HSET", KEYS[1], "raw", ARGV[1], "issued_at", ARGV[2])
redis.call("PEXPIRE", KEYS[1], ARGV[3])
return 1
`)

var openAITurnStateDeleteScript = redis.NewScript(`
if redis.call("HGET", KEYS[1], "raw") == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

var openAITurnStateReleaseLeaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

type openAITurnStateStore struct {
	rdb *redis.Client
	now func() time.Time
}

func NewOpenAITurnStateStore(rdb *redis.Client) service.OpenAITurnStateStore {
	return &openAITurnStateStore{rdb: rdb, now: time.Now}
}

func (s *openAITurnStateStore) Get(ctx context.Context, key service.OpenAITurnStateKey, now time.Time) (service.OpenAITurnStateTicket, bool, error) {
	if s == nil || s.rdb == nil {
		return service.OpenAITurnStateTicket{}, false, errors.New("OpenAI turn-state Redis client is unavailable")
	}
	redisKey, err := openAITurnStateRedisKey(key)
	if err != nil {
		return service.OpenAITurnStateTicket{}, false, err
	}
	values, err := s.rdb.HGetAll(ctx, redisKey).Result()
	if err != nil || len(values) == 0 {
		return service.OpenAITurnStateTicket{}, false, err
	}
	ticket, err := service.ParseOpenAITurnStateReuseTicket(values["raw"], now)
	if err != nil {
		_ = s.rdb.Del(ctx, redisKey).Err()
		return service.OpenAITurnStateTicket{}, false, nil
	}
	issuedAt, err := strconv.ParseInt(values["issued_at"], 10, 64)
	if err != nil || issuedAt != ticket.IssuedAt.Unix() {
		_ = s.rdb.Del(ctx, redisKey).Err()
		return service.OpenAITurnStateTicket{}, false, nil
	}
	return ticket, true, nil
}

func (s *openAITurnStateStore) Put(ctx context.Context, key service.OpenAITurnStateKey, ticket service.OpenAITurnStateTicket) (bool, error) {
	if s == nil || s.rdb == nil {
		return false, errors.New("OpenAI turn-state Redis client is unavailable")
	}
	redisKey, err := openAITurnStateRedisKey(key)
	if err != nil {
		return false, err
	}
	ttl := ticket.ExpiresAt.Sub(s.now())
	if ticket.Raw == "" || ticket.IssuedAt.IsZero() || ttl <= 0 {
		return false, nil
	}
	result, err := openAITurnStatePutScript.Run(ctx, s.rdb, []string{redisKey}, ticket.Raw, ticket.IssuedAt.Unix(), maxDurationMillis(ttl)).Int64()
	return result == 1, err
}

func (s *openAITurnStateStore) DeleteIfMatch(ctx context.Context, key service.OpenAITurnStateKey, rawTicket string) (bool, error) {
	if s == nil || s.rdb == nil {
		return false, errors.New("OpenAI turn-state Redis client is unavailable")
	}
	redisKey, err := openAITurnStateRedisKey(key)
	if err != nil {
		return false, err
	}
	result, err := openAITurnStateDeleteScript.Run(ctx, s.rdb, []string{redisKey}, rawTicket).Int64()
	return result == 1, err
}

func (s *openAITurnStateStore) AcquireLease(ctx context.Context, key service.OpenAITurnStateKey, owner string, ttl time.Duration) (bool, error) {
	if s == nil || s.rdb == nil {
		return false, errors.New("OpenAI turn-state Redis client is unavailable")
	}
	leaseKey, err := openAITurnStateLeaseKey(key)
	if err != nil {
		return false, err
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return false, errors.New("OpenAI turn-state lease owner is required")
	}
	return s.rdb.SetNX(ctx, leaseKey, owner, ttl).Result()
}

func (s *openAITurnStateStore) ReleaseLease(ctx context.Context, key service.OpenAITurnStateKey, owner string) error {
	if s == nil || s.rdb == nil {
		return errors.New("OpenAI turn-state Redis client is unavailable")
	}
	leaseKey, err := openAITurnStateLeaseKey(key)
	if err != nil {
		return err
	}
	return openAITurnStateReleaseLeaseScript.Run(ctx, s.rdb, []string{leaseKey}, owner).Err()
}

func openAITurnStateRedisKey(key service.OpenAITurnStateKey) (string, error) {
	if key.AccountID <= 0 || strings.TrimSpace(key.Model) == "" || strings.TrimSpace(key.CredentialHash) == "" {
		return "", errors.New("invalid OpenAI turn-state key")
	}
	return fmt.Sprintf("%sticket:%d:%s:%s", openAITurnStateKeyPrefix, key.AccountID, strings.TrimSpace(key.Model), strings.TrimSpace(key.CredentialHash)), nil
}

func openAITurnStateLeaseKey(key service.OpenAITurnStateKey) (string, error) {
	value, err := openAITurnStateRedisKey(key)
	if err != nil {
		return "", err
	}
	return strings.Replace(value, ":ticket:", ":lease:", 1), nil
}

func maxDurationMillis(ttl time.Duration) int64 {
	if ttl < time.Millisecond {
		return 1
	}
	return ttl.Milliseconds()
}

var _ service.OpenAITurnStateStore = (*openAITurnStateStore)(nil)
