package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"sync"
	"time"
)

const harvestCoordinationKey = "codex:harvest:execution"
const harvestCoordinationTTL = 60 * time.Second

var releaseHarvestLease = redis.NewScript(`if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('DEL',KEYS[1]) end return 0`)
var refreshHarvestLease = redis.NewScript(`if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('PEXPIRE',KEYS[1],ARGV[2]) end return 0`)

func (s *OpenAIGatewayService) acquireHarvestCoordination(ctx context.Context) (context.Context, func(), error) {
	if s.coordinationRedis == nil {
		return ctx, func() {}, nil
	}
	owner := uuid.NewString()
	query, stop := context.WithTimeout(ctx, 2*time.Second)
	ok, err := s.coordinationRedis.SetNX(query, harvestCoordinationKey, owner, harvestCoordinationTTL).Result()
	stop()
	if err != nil {
		return ctx, nil, errors.New("harvest coordination unavailable")
	}
	if !ok {
		return ctx, nil, errors.New("another instance is harvesting")
	}
	run, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-run.Done():
				return
			case <-ticker.C:
				q, finish := context.WithTimeout(run, 2*time.Second)
				n, e := refreshHarvestLease.Run(q, s.coordinationRedis, []string{harvestCoordinationKey}, owner, harvestCoordinationTTL.Milliseconds()).Int()
				finish()
				if e != nil || n != 1 {
					cancel()
					return
				}
			}
		}
	}()
	var once sync.Once
	return run, func() {
		once.Do(func() {
			cancel()
			<-done
			q, finish := context.WithTimeout(context.Background(), 2*time.Second)
			defer finish()
			_ = releaseHarvestLease.Run(q, s.coordinationRedis, []string{harvestCoordinationKey}, owner).Err()
		})
	}, nil
}
func (s *OpenAIGatewayService) holdDistributedCodexChat(accountID int64) func() {
	if s.coordinationRedis == nil {
		return func() {}
	}
	key := fmt.Sprintf("codex:harvest:active-chat:%d", accountID)
	member := uuid.NewString()
	renew := func() {
		q, done := context.WithTimeout(context.Background(), time.Second)
		defer done()
		pipe := s.coordinationRedis.TxPipeline()
		pipe.ZAdd(q, key, redis.Z{Score: float64(time.Now().Add(90 * time.Second).UnixMilli()), Member: member})
		pipe.Expire(q, key, 120*time.Second)
		_, _ = pipe.Exec(q)
	}
	renew()
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				renew()
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(stop)
			<-done
			q, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = s.coordinationRedis.ZRem(q, key, member).Err()
		})
	}
}
func (s *OpenAIGatewayService) distributedCodexChatHeld(accountID int64) bool {
	if s.coordinationRedis == nil {
		return false
	}
	q, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	key := fmt.Sprintf("codex:harvest:active-chat:%d", accountID)
	pipe := s.coordinationRedis.TxPipeline()
	pipe.ZRemRangeByScore(q, key, "-inf", fmt.Sprint(time.Now().UnixMilli()))
	count := pipe.ZCard(q, key)
	_, err := pipe.Exec(q)
	// On uncertainty, skip background probes rather than disturb business traffic.
	if err != nil {
		return true
	}
	if count.Val() > 0 {
		return true
	}
	if s.concurrencyService != nil {
		counts, err := s.concurrencyService.GetAccountConcurrencyBatch(q, []int64{accountID})
		return err != nil || counts[accountID] > 0
	}
	return false
}
