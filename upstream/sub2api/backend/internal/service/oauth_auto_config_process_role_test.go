package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type requestLocalConcurrencyRepo struct{ received chan AccountConcurrencyResult }

func (r requestLocalConcurrencyRepo) RecordConcurrencyResult(_ context.Context, result AccountConcurrencyResult, _ OAuthAutoConfig) error {
	r.received <- result
	return nil
}
func TestAutoConfigAPIRoleConsumesRequestLocalResults(t *testing.T) {
	cfg := DefaultOAuthAutoConfig()
	cfg.UpgradeEnabled = true
	cfg.UpgradeGroupIDs = []int64{1}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	svc := NewAccountOpsService(&accountOpsSettingsStub{raw: string(raw)}, &accountOpsRepoStub{}, nil)
	received := make(chan AccountConcurrencyResult, 1)
	svc.autoAccounts = requestLocalConcurrencyRepo{received: received}
	svc.start(false) // API role: no singleton mail delivery, but request outcomes are local.
	defer svc.Stop()
	require.Eventually(t, func() bool { c, _ := svc.autoConfig.Load().(OAuthAutoConfig); return c.UpgradeEnabled }, time.Second, time.Millisecond)
	svc.ObserveConcurrencyResult(AccountConcurrencyResult{AccountID: 427, Success: true, StartedAt: time.Now()})
	select {
	case result := <-received:
		require.Equal(t, int64(427), result.AccountID)
	case <-time.After(time.Second):
		t.Fatal("API request result was not consumed")
	}
}
