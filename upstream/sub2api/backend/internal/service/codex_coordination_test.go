package service

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCodexHarvestCoordinationAcrossInstances(t *testing.T) {
	server := miniredis.RunT(t)
	db := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer db.Close()
	a, b := &OpenAIGatewayService{coordinationRedis: db}, &OpenAIGatewayService{coordinationRedis: db}
	account := &Account{ID: 42}
	one := a.holdCodexTicketChat(account)
	two := a.holdCodexTicketChat(account)
	require.True(t, b.codexTicketChatHeld(42))
	one()
	require.True(t, b.codexTicketChatHeld(42))
	two()
	require.False(t, b.codexTicketChatHeld(42))
	_, release, err := a.acquireHarvestCoordination(context.Background())
	require.NoError(t, err)
	_, _, err = b.acquireHarvestCoordination(context.Background())
	require.Error(t, err)
	release()
	_, release, err = b.acquireHarvestCoordination(context.Background())
	require.NoError(t, err)
	release()
}
