package service

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const guardRuntimeKey = "account-token-guard:runtime"
const harvestRuntimeKey = "codex:harvest:runtime:worker"

func (s *AccountTokenGuardService) publishRuntime() {
	if s.runtimeRedis == nil {
		return
	}
	snapshot := s.localRuntimeInfo()
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ttl := 90 * time.Second
	if !snapshot.Running {
		ttl = 48 * time.Hour
	}
	_ = s.runtimeRedis.Set(ctx, guardRuntimeKey, raw, ttl).Err()
}
func (s *AccountTokenGuardService) startRuntimePublisher() func() {
	if s.runtimeRedis == nil {
		return func() {}
	}
	s.publishRuntime()
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(20 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				s.publishRuntime()
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(stop); <-done }) }
}
func (s *AccountTokenGuardService) runtimeInfo() AccountTokenGuardRuntime {
	if s.runtimeRedis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		raw, err := s.runtimeRedis.Get(ctx, guardRuntimeKey).Bytes()
		var value AccountTokenGuardRuntime
		if err == nil && json.Unmarshal(raw, &value) == nil {
			return value
		}
	}
	return s.localRuntimeInfo()
}
