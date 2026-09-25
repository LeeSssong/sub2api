package service

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestFusionWorkerRuntimeVisibleOnAPI(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	worker := NewAccountTokenGuardService(nil, nil, nil, nil, nil)
	api := NewAccountTokenGuardService(nil, nil, nil, nil, nil)
	worker.runtimeRedis = client
	api.runtimeRedis = client
	worker.finishCycle(time.Now(), AccountTokenGuardStats{Probed: 4}, "finished")
	worker.publishRuntime()
	require.Equal(t, 4, api.runtimeInfo().Stats.Probed)
	require.Equal(t, "finished", api.runtimeInfo().LastMessage)
	harvestWorker := &CodexHarvestService{runtimeRedis: client, runtimePublisher: true}
	harvestAPI := &CodexHarvestService{runtimeRedis: client}
	harvestWorker.setRuntime(func(r *CodexHarvestRuntime) { r.RequestsUsed = 3; r.RequestBudget = 6 })
	require.Equal(t, 3, harvestAPI.Runtime().RequestsUsed)
}
