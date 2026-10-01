//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/oauthobs"
	"github.com/stretchr/testify/require"
)

// Uses the complete native migration chain, not the reduced SQL fixture. It
// catches native column/trigger incompatibilities and concurrent projection loss.
func TestOAuthObservationNativeSchemaRoundTrip(t *testing.T) {
	ctx := context.Background()
	var accountID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type,status,schedulable,concurrency,credentials)
 VALUES('oauth-observation-integration','openai','oauth','active',true,15,'{"chatgpt_account_id":"test-workspace","chatgpt_user_id":"test-subject"}') RETURNING id`).Scan(&accountID))
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, accountID) })
	r := newAccountRepositoryWithSQL(nil, integrationDB, nil)
	at := time.Now().UTC().Truncate(time.Microsecond)
	event := func(key, verdict string, when time.Time) oauthobs.Event {
		return oauthobs.Event{AccountID: accountID, Key: fmt.Sprintf("integration-%d-%s", accountID, key), OccurredAt: when, Type: "probe_result", Payload: oauthobs.Payload{Model: "gpt-6-astra", Protocol: "native", ProbeVersion: "turn_state_v1", Verdict: verdict}}
	}
	require.NoError(t, r.WriteOAuthObservations(ctx, []oauthobs.Event{event("healthy", "healthy", at)}))
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, e := range []oauthobs.Event{event("late", "degraded", at.Add(2*time.Minute)), event("early", "degraded", at.Add(time.Minute))} {
		wg.Add(1)
		go func(e oauthobs.Event) { defer wg.Done(); errs <- r.WriteOAuthObservations(ctx, []oauthobs.Event{e}) }(e)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var healthy, degraded time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT first_healthy_at,first_degraded_at FROM oauth_observation_episode_lifetimes WHERE episode_account_id=$1`, accountID).Scan(&healthy, &degraded))
	require.True(t, healthy.Equal(at))
	require.True(t, degraded.Equal(at.Add(time.Minute)))
	var planID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO scheduled_test_plans(account_id,model_id,cron_expression,enabled,pelican_config)
 VALUES($1,'gpt-6-astra','*/2 * * * *',true,'{"question_kind":"state_probe","quality":{"action":"disable_scheduling"}}') RETURNING id`, accountID).Scan(&planID))
	until := time.Now().Add(time.Hour)
	_, err := integrationDB.ExecContext(ctx, `UPDATE scheduled_test_plans SET running_until=$2 WHERE id=$1`, planID, until)
	require.NoError(t, err)
	p, err := NewScheduledTestPlanRepository(integrationDB).GetByID(ctx, planID)
	require.NoError(t, err)
	action, err := NewScheduledTestPlanRepository(integrationDB).ApplyQualityOutcome(ctx, p, until, "failed")
	require.NoError(t, err)
	require.Equal(t, "scheduling_disabled", action)
	var attributed bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM oauth_observation_archives WHERE subject_type='account' AND subject_key=$1 AND snapshot->>'source'='quality_policy' AND snapshot->'after'->>'schedulable'='false')`, fmt.Sprint(accountID)).Scan(&attributed))
	require.True(t, attributed)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE id=$1`, planID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, accountID)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT first_healthy_at,first_degraded_at FROM oauth_observation_episode_lifetimes WHERE episode_account_id=$1`, accountID).Scan(&healthy, &degraded))
	require.True(t, degraded.Equal(at.Add(time.Minute)))
}
