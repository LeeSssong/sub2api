package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const quality5xxSource = "upstream_5xx"
const quality5xxPendingKey = "quality:5xx:pending"

type qualityProbeContextKey struct{}

// Redis bridges request-serving replicas and the existing singleton test worker.
// Only account IDs enter this bounded, coalescing queue; never upstream bodies.
type quality5xxTrigger struct {
	redis      *redis.Client
	ownsClient bool
}

func newQuality5xxTrigger(shared *redis.Client) *quality5xxTrigger {
	if shared == nil {
		return nil
	}
	// Same Redis database and credentials, separate tiny pool so stalled signal
	// delivery cannot hold a user request behind an unrelated shared-pool wait.
	opts := *shared.Options()
	opts.ContextTimeoutEnabled = true
	opts.ReadTimeout = 150 * time.Millisecond
	opts.WriteTimeout = 150 * time.Millisecond
	opts.DialTimeout = 150 * time.Millisecond
	opts.PoolTimeout = 150 * time.Millisecond
	opts.MaxRetries = -1
	opts.PoolSize = 4
	opts.MinIdleConns = 0
	return &quality5xxTrigger{redis: redis.NewClient(&opts), ownsClient: true}
}

var queueQuality5xx = redis.NewScript(`
 if redis.call('ZSCORE', KEYS[1], ARGV[1]) then return 0 end
 if not redis.call('SET', KEYS[2], '1', 'NX', 'EX', 60) then return 0 end
 redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
 redis.call('EXPIRE', KEYS[1], 1200)
 return 1`)

func (s *quality5xxTrigger) Observe(ctx context.Context, account *Account, status int) {
	if s == nil || s.redis == nil || account == nil || account.ID <= 0 || account.Type != AccountTypeOAuth || status < 500 || status > 599 || ctx.Value(qualityProbeContextKey{}) != nil {
		return
	}
	enqueueCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 150*time.Millisecond)
	defer cancel()
	id := strconv.FormatInt(account.ID, 10)
	if _, err := queueQuality5xx.Run(enqueueCtx, s.redis, []string{quality5xxPendingKey, "quality:5xx:cooldown:" + id}, id, time.Now().UnixMilli()).Result(); err != nil {
		slog.Warn("quality 5xx trigger could not be queued", "account_id", account.ID)
	}
}

func (s *ScheduledTestRunnerService) startQualityTriggers() {
	if s.qualityTrigger == nil || s.qualityTrigger.redis == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.triggerCancel = cancel
	s.triggerWG.Add(1)
	go func() {
		defer s.triggerWG.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var active sync.Map
		slots := make(chan struct{}, scheduledTestDefaultMaxWorkers)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			scanCtx, stop := context.WithTimeout(ctx, 2*time.Second)
			rdb := s.qualityTrigger.redis
			_ = rdb.ZRemRangeByScore(scanCtx, quality5xxPendingKey, "-inf", strconv.FormatInt(time.Now().Add(-20*time.Minute).UnixMilli(), 10)).Err()
			pending, err := rdb.ZRangeWithScores(scanCtx, quality5xxPendingKey, 0, 99).Result()
			stop()
			if err != nil {
				continue
			}
			for _, entry := range pending {
				id, err := strconv.ParseInt(fmt.Sprint(entry.Member), 10, 64)
				if err != nil {
					continue
				}
				if _, loaded := active.LoadOrStore(id, true); loaded {
					continue
				}
				select {
				case slots <- struct{}{}:
				default:
					active.Delete(id)
					continue
				}
				s.triggerWG.Add(1)
				go func(id int64, entry redis.Z) {
					defer s.triggerWG.Done()
					defer func() { <-slots; active.Delete(id) }()
					runCtx, cancel := context.WithTimeout(ctx, 12*time.Minute)
					defer cancel()
					if err := s.runQualityTriggeredAccount(runCtx, id); err != nil {
						return
					}
					if runCtx.Err() != nil {
						return
					}
					// Do not remove a newer signal that arrived after queue expiry.
					_ = rdb.Eval(runCtx, `if tonumber(redis.call('ZSCORE',KEYS[1],ARGV[1])) == tonumber(ARGV[2]) then return redis.call('ZREM',KEYS[1],ARGV[1]) end return 0`, []string{quality5xxPendingKey}, entry.Member, entry.Score).Err()
				}(id, entry)
			}
		}
	}()
}

func (s *ScheduledTestRunnerService) runQualityTriggeredAccount(ctx context.Context, accountID int64) error {
	plans, err := s.planRepo.ListByAccountID(ctx, accountID)
	if err != nil {
		return err
	}
	for _, plan := range plans {
		if !plan.Enabled || plan.PelicanConfig == nil || plan.PelicanConfig.Quality == nil || !plan.PelicanConfig.Quality.TriggerOnUpstream5xx {
			continue
		}
		plan.TriggerSource = quality5xxSource
		if plan.RunningUntil != nil && plan.RunningUntil.After(time.Now()) {
			return fmt.Errorf("quality plan %d is still running", plan.ID)
		}
		if plan.LastRunAt != nil && plan.LastRunAt.After(time.Now().Add(-time.Minute)) {
			continue
		}
		if !s.runOnePlan(ctx, plan) {
			// A lost claim can mean an actual running test OR only a transient
			// advisory-lock conflict. Re-read before acknowledging this signal.
			current, err := s.planRepo.GetByID(ctx, plan.ID)
			if err != nil {
				return err
			}
			if current == nil || !current.Enabled || current.PelicanConfig == nil || current.PelicanConfig.Quality == nil || !current.PelicanConfig.Quality.TriggerOnUpstream5xx {
				continue
			}
			if current.RunningUntil != nil && current.RunningUntil.After(time.Now()) {
				return fmt.Errorf("quality plan %d is still running", plan.ID)
			}
			if current.LastRunAt != nil && current.LastRunAt.After(time.Now().Add(-time.Minute)) {
				continue
			}
			return fmt.Errorf("quality plan %d could not be claimed", plan.ID)
		}
	}
	return nil
}

// Called immediately after real transport responses, before protocol retry or
// status mapping. Transport errors and HTTP-200 stream failures are excluded.
func (s *RateLimitService) observeQualityResponse(ctx context.Context, account *Account, response *http.Response, err error) {
	if s == nil || err != nil || response == nil {
		return
	}
	s.qualityTrigger.Observe(ctx, account, response.StatusCode)
}
