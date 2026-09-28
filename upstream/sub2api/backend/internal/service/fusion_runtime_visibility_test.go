package service

import (
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHarvestWorkerRuntimeVisibleOnAPI(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer client.Close()
	harvestWorker := &CodexHarvestService{runtimeRedis: client, runtimePublisher: true}
	harvestAPI := &CodexHarvestService{runtimeRedis: client}
	harvestWorker.setRuntime(func(r *CodexHarvestRuntime) { r.RequestsUsed = 3; r.RequestBudget = 6 })
	require.Equal(t, 3, harvestAPI.Runtime().RequestsUsed)
}
