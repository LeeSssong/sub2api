//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestExcelBPSRecoveryAtomicLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
	a := newExcelBPSAutoDisableAccount()
	a.Extra[service.ExcelBPSShadowRecoveryKey] = true
	a.Extra[service.ExcelBPSFallbackModelsKey] = []string{"gpt-6-sol"}
	a = mustCreateAccount(t, integrationEntClient, a)
	t.Cleanup(func() { _, _ = integrationDB.Exec("DELETE FROM accounts WHERE id=$1", a.ID) })
	var wg sync.WaitGroup
	var mu sync.Mutex
	changed := 0
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.DegradeExcelBPS(ctx, a, 503)
			require.NoError(t, err)
			if ok {
				mu.Lock()
				changed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 1, changed)
	load := func() *service.Account { a, err := repo.GetByID(ctx, a.ID); require.NoError(t, err); return a }
	require.True(t, load().IsExcelBPSConfigured())
	require.False(t, load().IsExcelBPSEnabled())
	now := time.Now().Add(6 * time.Minute)
	claims := make(chan *service.Account, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := repo.ClaimDueExcelBPSRecoveries(ctx, now, 3)
			require.NoError(t, err)
			for _, a := range got {
				claims <- a
			}
		}()
	}
	wg.Wait()
	close(claims)
	var claim *service.Account
	count := 0
	for a := range claims {
		claim = a
		count++
	}
	require.Equal(t, 1, count)
	// Routine usage/observation writes must not keep a busy healthy account degraded.
	_, err := integrationDB.Exec("UPDATE accounts SET extra=extra || '{\"codex_5h_used_percent\":12}', last_used_at=NOW(), updated_at=NOW() WHERE id=$1", a.ID)
	require.NoError(t, err)
	ok, err := repo.FinishExcelBPSRecovery(ctx, claim, false, now)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, load().ExcelBPSRecovery().Failures)
	require.WithinDuration(t, now.Add(10*time.Minute), load().ExcelBPSRecovery().NextProbeAt, time.Second)
	// Lease expiry simulates a crashed instance; a new claimant can resume.
	got, err := repo.ClaimDueExcelBPSRecoveries(ctx, now.Add(11*time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, got, 1)
	old := got[0]
	got, err = repo.ClaimDueExcelBPSRecoveries(ctx, now.Add(16*time.Minute), 3)
	require.NoError(t, err)
	require.Len(t, got, 1)
	ok, err = repo.FinishExcelBPSRecovery(ctx, old, true, now.Add(16*time.Minute))
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = repo.FinishExcelBPSRecovery(ctx, got[0], true, now.Add(16*time.Minute))
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, load().IsExcelBPSEnabled())
	ok, err = repo.DegradeExcelBPS(ctx, a, 503)
	require.NoError(t, err)
	require.False(t, ok, "late failure from pre-degradation request cannot overwrite a recovered generation")
}

func TestExcelBPSRecoveryRejectsStaleAcceptance(t *testing.T) {
	for name, mutation := range map[string]string{
		"credentials":   `credentials='{"access_token":"replacement"}'`,
		"manual off":    `extra=extra || '{"openai_excel_bps":false}'`,
		"shadow off":    `extra=extra || '{"openai_excel_bps_shadow_recovery":false}'`,
		"models edited": `extra=extra || '{"openai_excel_bps_fallback_models":["other"]}'`,
		"status paused": `status='disabled'`, "unschedulable": `schedulable=false`,
		"cooldown": `temp_unschedulable_until=NOW()+interval '1 hour'`,
		"deleted":  `deleted_at=NOW()`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			repo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
			input := newExcelBPSAutoDisableAccount()
			input.Extra[service.ExcelBPSShadowRecoveryKey] = true
			input.Extra[service.ExcelBPSFallbackModelsKey] = []string{"gpt-6-sol"}
			a := mustCreateAccount(t, integrationEntClient, input)
			defer integrationDB.Exec("DELETE FROM accounts WHERE id=$1", a.ID)
			ok, err := repo.DegradeExcelBPS(ctx, a, 403)
			require.NoError(t, err)
			require.True(t, ok)
			now := time.Now().Add(6 * time.Minute)
			got, err := repo.ClaimDueExcelBPSRecoveries(ctx, now, 3)
			require.NoError(t, err)
			require.Len(t, got, 1)
			_, err = integrationDB.Exec(fmt.Sprintf("UPDATE accounts SET %s WHERE id=$1", mutation), a.ID)
			require.NoError(t, err)
			ok, err = repo.FinishExcelBPSRecovery(ctx, got[0], true, now)
			require.NoError(t, err)
			require.False(t, ok)
		})
	}
}

func TestExcelBPSRecoveryNonNormalNotClaimed(t *testing.T) {
	for name, mutation := range map[string]string{
		"disabled": `status='disabled'`, "unschedulable": `schedulable=false`, "admission": `extra=extra || '{"account_admission_blocked":true}'`,
		"expired": `auto_pause_on_expired=true,expires_at=NOW()-interval '1 minute'`, "rate limit": `rate_limit_reset_at=NOW()+interval '1 hour'`, "overload": `overload_until=NOW()+interval '1 hour'`, "temp cooldown": `temp_unschedulable_until=NOW()+interval '1 hour'`, "deleted": `deleted_at=NOW()`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			repo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
			input := newExcelBPSAutoDisableAccount()
			input.Extra[service.ExcelBPSShadowRecoveryKey] = true
			a := mustCreateAccount(t, integrationEntClient, input)
			defer integrationDB.Exec("DELETE FROM accounts WHERE id=$1", a.ID)
			changed, err := repo.DegradeExcelBPS(ctx, a, 503)
			require.NoError(t, err)
			require.True(t, changed)
			_, err = integrationDB.Exec("UPDATE accounts SET "+mutation+" WHERE id=$1", a.ID)
			require.NoError(t, err)
			claims, err := repo.ClaimDueExcelBPSRecoveries(ctx, time.Now().Add(6*time.Minute), 3)
			require.NoError(t, err)
			require.Empty(t, claims)
		})
	}
}

func TestExcelBPSRecoveryAdminPreservationAndLegacyDegrade(t *testing.T) {
	ctx := context.Background()
	repo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
	for _, status := range []int{403, 500, 503, 599, 429, 400} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			input := newExcelBPSAutoDisableAccount()
			delete(input.Extra, "openai_excel_bps_auto_disable_on_403")
			a := mustCreateAccount(t, integrationEntClient, input)
			defer integrationDB.Exec("DELETE FROM accounts WHERE id=$1", a.ID)
			changed, err := repo.DegradeExcelBPS(ctx, a, status)
			require.NoError(t, err)
			require.Equal(t, service.IsExcelBPSDegradationStatus(status), changed)
			after, err := repo.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.Equal(t, !changed, after.IsExcelBPSEnabled())
		})
	}
	input := newExcelBPSAutoDisableAccount()
	input.Extra[service.ExcelBPSShadowRecoveryKey] = true
	input.Extra[service.ExcelBPSFallbackModelsKey] = []string{"normal"}
	a := mustCreateAccount(t, integrationEntClient, input)
	defer integrationDB.Exec("DELETE FROM accounts WHERE id=$1", a.ID)
	changed, err := repo.DegradeExcelBPS(ctx, a, 403)
	require.NoError(t, err)
	require.True(t, changed)
	// A stale edit cannot forge away the server-owned degradation state.
	a.Extra[service.ExcelBPSRecoveryKey] = map[string]any{"active": false}
	a.Extra[service.ExcelBPSShadowRecoveryKey] = false
	require.NoError(t, repo.Update(ctx, a))
	a, err = repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, a.IsExcelBPSDegraded())
	require.False(t, a.IsExcelBPSShadowRecoveryEnabled())
	require.NoError(t, repo.UpdateExtra(ctx, a.ID, map[string]any{service.ExcelBPSRecoveryKey: map[string]any{"active": false}}))
	a, err = repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, a.IsExcelBPSDegraded())
	require.NoError(t, repo.UpdateExtra(ctx, a.ID, map[string]any{"openai_excel_bps": false}))
	a, err = repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.NotContains(t, a.Extra, service.ExcelBPSRecoveryKey)
}

func TestExcelBPSRecoverySameProxyTransportGuard(t *testing.T) {
	for _, phase := range []string{"degrade", "finish"} {
		for _, edit := range []string{"unchanged", "metadata", "host", "password"} {
			t.Run(phase+"/"+edit, func(t *testing.T) {
				ctx := context.Background()
				repo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
				proxy := mustCreateProxy(t, integrationEntClient, &service.Proxy{Name: "recovery-proxy", Protocol: "http", Host: "before.invalid", Port: 8080, Username: "test-user", Password: "test-before", Status: service.StatusActive})
				defer integrationDB.Exec("DELETE FROM proxies WHERE id=$1", proxy.ID)
				input := newExcelBPSAutoDisableAccount()
				input.ProxyID = &proxy.ID
				input.Extra[service.ExcelBPSShadowRecoveryKey] = true
				input.Extra[service.ExcelBPSFallbackModelsKey] = []string{"normal"}
				a := mustCreateAccount(t, integrationEntClient, input)
				defer integrationDB.Exec("DELETE FROM accounts WHERE id=$1", a.ID)
				a, err := repo.GetByID(ctx, a.ID)
				require.NoError(t, err)
				now := time.Now().Add(6 * time.Minute)
				if phase == "finish" {
					changed, err := repo.DegradeExcelBPS(ctx, a, 503)
					require.NoError(t, err)
					require.True(t, changed)
					claims, err := repo.ClaimDueExcelBPSRecoveries(ctx, now, 3)
					require.NoError(t, err)
					require.Len(t, claims, 1)
					a = claims[0]
				}
				queries := map[string]string{"metadata": "name='renamed',updated_at=NOW()", "host": "host='after.invalid',updated_at=NOW()", "password": "password='test-after',updated_at=NOW()"}
				if edit != "unchanged" {
					_, err = integrationDB.Exec("UPDATE proxies SET "+queries[edit]+" WHERE id=$1", proxy.ID)
					require.NoError(t, err)
				}
				var changed bool
				if phase == "finish" {
					changed, err = repo.FinishExcelBPSRecovery(ctx, a, true, now)
				} else {
					changed, err = repo.DegradeExcelBPS(ctx, a, 503)
				}
				require.NoError(t, err)
				require.Equal(t, edit == "unchanged" || edit == "metadata", changed)
			})
		}
	}
}

func TestExcelBPSRecoveryWhitelistReplacementAndPatch(t *testing.T) {
	ctx := context.Background()
	repo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
	input := newExcelBPSAutoDisableAccount()
	input.Extra[service.ExcelBPSShadowRecoveryKey] = true
	input.Extra[service.ExcelBPSFallbackModelsKey] = []string{"normal-only"}
	a := mustCreateAccount(t, integrationEntClient, input)
	defer integrationDB.Exec("DELETE FROM accounts WHERE id=$1", a.ID)
	changed, err := repo.DegradeExcelBPS(ctx, a, 503)
	require.NoError(t, err)
	require.True(t, changed)
	a, err = repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	a.Extra = map[string]any{"openai_excel_bps": true, service.ExcelBPSShadowRecoveryKey: true}
	require.NoError(t, repo.Update(ctx, a))
	a, err = repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, a.IsExcelBPSDegraded())
	require.True(t, a.IsModelSupported("normal-only"))
	require.False(t, a.IsModelSupported("unlisted"))
	_, err = repo.BulkUpdate(ctx, []int64{a.ID}, service.AccountBulkUpdate{Extra: map[string]any{service.ExcelBPSShadowRecoveryKey: false}})
	require.NoError(t, err)
	a, err = repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, a.IsExcelBPSDegraded())
	require.False(t, a.IsExcelBPSShadowRecoveryEnabled())
	require.True(t, a.IsModelSupported("normal-only"))
	require.False(t, a.IsModelSupported("unlisted"))
	require.NoError(t, repo.UpdateExtra(ctx, a.ID, map[string]any{"openai_excel_bps_ignore_images": true}))
	a, err = repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, a.IsModelSupported("normal-only"))
	require.False(t, a.IsModelSupported("unlisted"))
}
