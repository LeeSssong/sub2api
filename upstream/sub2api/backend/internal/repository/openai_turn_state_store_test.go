package repository

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func repositoryTurnStateTicket(decodedLen int, issued time.Time) string {
	raw := make([]byte, decodedLen)
	raw[0] = 0x80
	binary.BigEndian.PutUint64(raw[1:9], uint64(issued.Unix()))
	return base64.URLEncoding.EncodeToString(raw)
}

func TestOpenAITurnStateStoreKeepsNewestAndIsolatesCredential(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewOpenAITurnStateStore(client).(*openAITurnStateStore)
	now := time.Unix(1_800_000_000, 0)
	store.now = func() time.Time { return now }
	key := service.OpenAITurnStateKey{AccountID: 7, Model: service.OpenAITurnStateHarvestModel, CredentialHash: "hash-a"}
	newer, err := service.ParseOpenAITurnStateReuseTicket(repositoryTurnStateTicket(217, now.Add(-time.Minute)), now)
	require.NoError(t, err)
	older, err := service.ParseOpenAITurnStateReuseTicket(repositoryTurnStateTicket(217, now.Add(-2*time.Minute)), now)
	require.NoError(t, err)

	ok, err := store.Put(context.Background(), key, newer)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = store.Put(context.Background(), key, older)
	require.NoError(t, err)
	require.False(t, ok)
	got, ok, err := store.Get(context.Background(), key, now)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, newer.Raw, got.Raw)

	_, ok, err = store.Get(context.Background(), service.OpenAITurnStateKey{AccountID: 7, Model: service.OpenAITurnStateHarvestModel, CredentialHash: "hash-b"}, now)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestOpenAITurnStateStoreConditionalDeleteAndLease(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewOpenAITurnStateStore(client).(*openAITurnStateStore)
	now := time.Unix(1_800_000_000, 0)
	store.now = func() time.Time { return now }
	key := service.OpenAITurnStateKey{AccountID: 8, Model: service.OpenAITurnStateHarvestModel, CredentialHash: "hash"}
	ticket, err := service.ParseOpenAITurnStateReuseTicket(repositoryTurnStateTicket(217, now.Add(-time.Minute)), now)
	require.NoError(t, err)
	_, err = store.Put(context.Background(), key, ticket)
	require.NoError(t, err)

	deleted, err := store.DeleteIfMatch(context.Background(), key, "stale")
	require.NoError(t, err)
	require.False(t, deleted)
	deleted, err = store.DeleteIfMatch(context.Background(), key, ticket.Raw)
	require.NoError(t, err)
	require.True(t, deleted)

	acquired, err := store.AcquireLease(context.Background(), key, "owner-a", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	acquired, err = store.AcquireLease(context.Background(), key, "owner-b", time.Minute)
	require.NoError(t, err)
	require.False(t, acquired)
	require.NoError(t, store.ReleaseLease(context.Background(), key, "owner-b"))
	acquired, err = store.AcquireLease(context.Background(), key, "owner-b", time.Minute)
	require.NoError(t, err)
	require.False(t, acquired)
	require.NoError(t, store.ReleaseLease(context.Background(), key, "owner-a"))
	acquired, err = store.AcquireLease(context.Background(), key, "owner-b", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
}
