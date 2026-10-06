package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPermanentAuthRecovery_WorkerReauthUpdatesServingAPISnapshot(t *testing.T) {
	workerReauth, reader, _, updater, _, _ := newReauthTestService("acct-1")
	api, workerGateway := &OpenAIGatewayService{}, &OpenAIGatewayService{}
	workerReauth.runtimeBlocker = workerGateway
	old := *reader.account
	api.BlockAccountScheduling(&old, time.Time{}, openAIPermanentAuthBlockReason)
	workerGateway.BlockAccountScheduling(&old, time.Time{}, openAIPermanentAuthBlockReason)
	ctx := context.Background()
	savePasswordReauthConfig(t, workerReauth)
	_, err := workerReauth.CreateTask(ctx, old.ID)
	require.NoError(t, err)
	claim, err := workerReauth.ClaimTask(ctx, "worker-a")
	require.NoError(t, err)
	_, err = workerReauth.SubmitCredentials(ctx, claim.TaskID, "worker-a", directReauthCredentials("acct-1", "user-1", "user@example.com"), nil)
	require.NoError(t, err)
	require.False(t, workerGateway.isOpenAIAccountRuntimeBlocked(&old))
	require.True(t, api.isOpenAIAccountRuntimeBlocked(&old), "worker recovery must not make old snapshots usable")
	// Model the account snapshot published after the atomic credentials update.
	recovered := old
	recovered.Credentials = updater.credentials
	recovered.Status, recovered.Schedulable = StatusActive, true
	require.False(t, api.isOpenAIAccountRequestRuntimeBlocked(&recovered, "gpt-5.5", false))
}

func TestPermanentAuthRecovery_ConcurrentStaleRecoveryKeepsLatestBlock(t *testing.T) {
	_, api, _, _, old := newPermanentAuthTestService()
	newer := *old
	newer.Credentials = map[string]any{"access_token": "latest-revoked", "_token_version": int64(300)}
	stale := *old
	stale.Credentials = map[string]any{"access_token": "stale-recovery", "_token_version": int64(200)}
	api.BlockAccountScheduling(&newer, time.Time{}, openAIPermanentAuthBlockReason)
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Requests own their Account snapshot, including its lazy model
			// mapping cache; only the gateway's runtime maps are shared.
			requestAccount := stale
			api.BlockAccountScheduling(old, time.Time{}, openAIPermanentAuthBlockReason)
			if !api.isOpenAIAccountRequestRuntimeBlocked(&requestAccount, "gpt-5.5", false) {
				t.Error("stale recovery cleared newer revocation")
			}
		}()
	}
	wg.Wait()
	require.True(t, api.isOpenAIAccountRuntimeBlocked(&newer))
}

func TestPermanentAuthRecovery_CrossProcessScheduler(t *testing.T) {
	_, api, _, _, old := newPermanentAuthTestService()
	old.Concurrency = 1
	api.BlockAccountScheduling(old, time.Time{}, openAIPermanentAuthBlockReason)
	recovered := *old
	recovered.Credentials = map[string]any{"access_token": "replacement-token", "_token_version": int64(100)}
	// The worker's local clear cannot reach the serving API's runtime map.
	worker := &OpenAIGatewayService{}
	worker.ClearAccountSchedulingBlock(old.ID)
	require.True(t, api.isOpenAIAccountRuntimeBlocked(old))
	api.accountRepo = schedulerTestOpenAIAccountRepo{accounts: []Account{recovered}}
	api.cache = &schedulerTestGatewayCache{}
	api.cfg = &config.Config{}
	api.concurrencyService = NewConcurrencyService(schedulerTestConcurrencyCache{})
	selected, _, err := api.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.5", nil, OpenAIUpstreamTransportAny, false)
	require.NoError(t, err)
	require.Equal(t, old.ID, selected.Account.ID)
	if selected.ReleaseFunc != nil {
		selected.ReleaseFunc()
	}
	require.False(t, api.isOpenAIAccountRuntimeBlocked(&recovered))
}

func TestPermanentAuthRecovery_RequiresNewHealthyCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, token, status                string
		version                            int64
		schedulable, cooldown, wantBlocked bool
	}{
		{"new credentials", "new", StatusActive, 200, true, false, false},
		{"status only", "revoked-test-token", StatusActive, 200, true, false, true},
		{"stale version", "new", StatusActive, 99, true, false, true},
		{"same version", "new", StatusActive, 100, true, false, true},
		{"missing version", "new", StatusActive, 0, true, false, true},
		{"missing token", "", StatusActive, 200, true, false, true},
		{"still error", "new", StatusError, 200, true, false, true},
		{"disabled", "new", StatusActive, 200, false, false, true},
		{"cooldown", "new", StatusActive, 200, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, requestGate := range []bool{false, true} {
				_, api, _, _, old := newPermanentAuthTestService()
				old.Credentials["_token_version"] = int64(100)
				api.BlockAccountScheduling(old, time.Time{}, openAIPermanentAuthBlockReason)
				recovered := *old
				recovered.Credentials = map[string]any{"access_token": tc.token, "_token_version": tc.version}
				recovered.Status, recovered.Schedulable = tc.status, tc.schedulable
				if tc.cooldown {
					until := time.Now().Add(time.Minute)
					recovered.TempUnschedulableUntil = &until
				}
				if requestGate {
					require.Equal(t, tc.wantBlocked, api.isOpenAIAccountRequestRuntimeBlocked(&recovered, "gpt-5.5", false))
				} else {
					require.Equal(t, tc.wantBlocked, api.isOpenAIAccountRuntimeBlocked(&recovered))
				}
			}
		})
	}
}

func TestPermanentAuthRecovery_OlderFailureCannotEraseNewerRevocation(t *testing.T) {
	_, api, _, _, old := newPermanentAuthTestService()
	old.Credentials["_token_version"] = int64(100)
	newer := *old
	newer.Credentials = map[string]any{"access_token": "new-revoked", "_token_version": int64(300)}
	api.BlockAccountScheduling(&newer, time.Time{}, openAIPermanentAuthBlockReason)
	api.BlockAccountScheduling(old, time.Time{}, openAIPermanentAuthBlockReason)
	intermediate := *old
	intermediate.Credentials = map[string]any{"access_token": "intermediate", "_token_version": int64(200)}
	require.True(t, api.isOpenAIAccountRequestRuntimeBlocked(&intermediate, "gpt-5.5", false))
	require.True(t, api.isOpenAIAccountRuntimeBlocked(&newer))
	recovered := newer
	recovered.Credentials = map[string]any{"access_token": "recovered", "_token_version": int64(400)}
	require.False(t, api.isOpenAIAccountRuntimeBlocked(&recovered))
}
