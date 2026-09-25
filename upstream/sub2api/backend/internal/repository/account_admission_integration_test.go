//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAdmissionDatabaseGate(t *testing.T) {
	ctx := context.Background()
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type,status,schedulable) VALUES('admission-gate','openai','apikey','active',false) RETURNING id`).Scan(&id))
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, id) })
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_admission_jobs(account_id,target_group_ids) VALUES($1,'{7}')`, id)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET schedulable=true,extra='{}' WHERE id=$1`, id)
	require.NoError(t, err)
	var schedulable, blocked bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT schedulable,(extra->>'account_admission_blocked')::boolean FROM accounts WHERE id=$1`, id).Scan(&schedulable, &blocked))
	require.False(t, schedulable, "old recovery paths must not release admission")
	require.True(t, blocked, "cached and shadow scheduling must retain the gate")
}

func admissionFixture(t *testing.T, temporary bool) (*accountRepository, *accountAdmissionRepository, *service.Account, int64, int64) {
	t.Helper()
	ctx := context.Background()
	var target, temp int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,platform,status) VALUES('admission-target','openai','active') RETURNING id`).Scan(&target))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,platform,status) VALUES('admission-temp','openai','active') RETURNING id`).Scan(&temp))
	repo := &accountRepository{client: integrationEntClient, sql: integrationDB}
	a := &service.Account{Name: "admission-fixture", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 1, Priority: 50, Credentials: map[string]any{"api_key": fmt.Sprintf("fixture-%d", target)}}
	var tmp *int64
	if temporary {
		tmp = &temp
	}
	require.NoError(t, repo.CreateWithAdmission(ctx, a, []int64{target}, tmp))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM scheduler_outbox WHERE account_id=$1`, a.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, a.ID)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id IN ($1,$2)`, target, temp)
	})
	return repo, &accountAdmissionRepository{db: integrationDB, accounts: repo}, a, target, temp
}
func admissionDue(t *testing.T, id int64) {
	t.Helper()
	_, err := integrationDB.Exec(`UPDATE account_admission_jobs SET next_run_at=NOW() WHERE account_id=$1`, id)
	require.NoError(t, err)
}
func admissionClaim(t *testing.T, r *accountAdmissionRepository) *service.AccountAdmissionJob {
	t.Helper()
	j, err := r.ClaimAdmission(context.Background())
	require.NoError(t, err)
	require.NotNil(t, j)
	return j
}
func admissionSuccess() *service.ScheduledTestResult {
	return &service.ScheduledTestResult{Status: "success", ResponseText: "fixture", StartedAt: time.Now(), FinishedAt: time.Now()}
}
func TestAdmissionLifecycleAndInconclusiveRecheck(t *testing.T) {
	for _, temporary := range []bool{false, true} {
		t.Run(fmt.Sprint(temporary), func(t *testing.T) {
			ctx := context.Background()
			accounts, r, a, target, temp := admissionFixture(t, temporary)
			got, err := accounts.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.False(t, got.Schedulable)
			require.True(t, got.AdmissionBlocked())
			if temporary {
				require.Equal(t, []int64{temp}, got.GroupIDs)
			} else {
				require.Empty(t, got.GroupIDs)
			}
			j := admissionClaim(t, r)
			other, err := r.ClaimAdmission(ctx)
			require.NoError(t, err)
			require.Nil(t, other, "a live lease is exclusive")
			require.NoError(t, r.FinishAdmission(ctx, j, "passed", admissionSuccess(), &service.ScheduledTestResult{Status: "failed", ErrorMessage: "context deadline exceeded"}))
			got, err = accounts.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.False(t, got.Schedulable)
			admissionDue(t, a.ID)
			j = admissionClaim(t, r)
			require.NoError(t, r.FinishAdmission(ctx, j, "passed", admissionSuccess(), admissionSuccess()))
			got, err = accounts.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.True(t, got.Schedulable)
			require.False(t, got.AdmissionBlocked())
			require.Equal(t, []int64{target}, got.GroupIDs)
			var seconds float64
			require.NoError(t, integrationDB.QueryRow(`SELECT EXTRACT(EPOCH FROM next_run_at-last_run_at) FROM account_admission_jobs WHERE account_id=$1`, a.ID).Scan(&seconds))
			require.Equal(t, float64(600), seconds)
			admissionDue(t, a.ID)
			j = admissionClaim(t, r)
			require.NoError(t, r.FinishAdmission(ctx, j, "inconclusive", admissionSuccess(), &service.ScheduledTestResult{Status: "failed", ErrorMessage: "timeout"}))
			got, err = accounts.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.True(t, got.Schedulable)
			require.Equal(t, []int64{target}, got.GroupIDs)
			admissionDue(t, a.ID)
			j = admissionClaim(t, r)
			require.NoError(t, r.FinishAdmission(ctx, j, "failed", &service.ScheduledTestResult{Status: "failed", ErrorMessage: "answer_mismatch: expected 21"}, admissionSuccess()))
			got, err = accounts.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.False(t, got.Schedulable)
			require.True(t, got.AdmissionBlocked())
			require.Empty(t, got.GroupIDs)
		})
	}
}
func TestAdmissionLeaseRecoveryRejectsStaleOwner(t *testing.T) {
	ctx := context.Background()
	accounts, r, a, _, _ := admissionFixture(t, false)
	old := admissionClaim(t, r)
	_, err := integrationDB.Exec(`UPDATE account_admission_jobs SET lease_until=NOW()-interval '1 second' WHERE account_id=$1`, a.ID)
	require.NoError(t, err)
	current := admissionClaim(t, r)
	require.NotEqual(t, old.LeaseToken, current.LeaseToken)
	require.NoError(t, r.FinishAdmission(ctx, old, "passed", admissionSuccess(), admissionSuccess()))
	got, err := accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.False(t, got.Schedulable)
	require.NoError(t, r.FinishAdmission(ctx, current, "passed", admissionSuccess(), admissionSuccess()))
	got, err = accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, got.Schedulable)
}
func TestAdmissionManualMembershipAndSameValuePauseWin(t *testing.T) {
	for _, kind := range []string{"membership", "pause"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			accounts, r, a, _, temp := admissionFixture(t, false)
			j := admissionClaim(t, r)
			if kind == "membership" {
				require.NoError(t, accounts.BindGroups(ctx, a.ID, []int64{temp}))
			} else {
				require.NoError(t, accounts.PauseAdmission(ctx, a.ID, false))
			}
			require.NoError(t, r.FinishAdmission(ctx, j, "passed", admissionSuccess(), admissionSuccess()))
			got, err := accounts.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.False(t, got.Schedulable)
			if kind == "membership" {
				require.Equal(t, []int64{temp}, got.GroupIDs)
			}
			var active bool
			require.NoError(t, integrationDB.QueryRow(`SELECT active FROM account_admission_jobs WHERE account_id=$1`, a.ID).Scan(&active))
			require.False(t, active)
		})
	}
}
func TestAdmissionDuplicateDoesNotDetachExisting(t *testing.T) {
	ctx := context.Background()
	accounts, r, a, target, _ := admissionFixture(t, false)
	j := admissionClaim(t, r)
	require.NoError(t, r.FinishAdmission(ctx, j, "passed", admissionSuccess(), admissionSuccess()))
	duplicate := *a
	duplicate.ID = 0
	require.ErrorIs(t, accounts.CreateWithAdmission(ctx, &duplicate, []int64{target}, nil), service.ErrAdmissionDuplicate)
	got, err := accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, got.Schedulable)
	require.Equal(t, []int64{target}, got.GroupIDs)
}
func TestAdmissionOrdinaryRefreshPreservesButManualCredentialsInvalidate(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(fmt.Sprint(manual), func(t *testing.T) {
			ctx := context.Background()
			accounts, r, a, _, _ := admissionFixture(t, false)
			j := admissionClaim(t, r)
			a.Credentials["access_token"] = "rotated-token"
			a.Credentials["_token_version"] = 123
			a.Credentials["expires_in"] = 3599
			if manual {
				require.NoError(t, accounts.Update(service.WithAdmissionCredentialEdit(ctx), a))
			} else {
				require.NoError(t, accounts.UpdateCredentials(ctx, a.ID, a.Credentials))
			}
			require.NoError(t, r.FinishAdmission(ctx, j, "passed", admissionSuccess(), admissionSuccess()))
			got, err := accounts.GetByID(ctx, a.ID)
			require.NoError(t, err)
			require.Equal(t, !manual, got.Schedulable)
		})
	}
}

func TestAdmissionExplicitEmptyMembershipCancelsRun(t *testing.T) {
	ctx := context.Background()
	accounts, r, a, _, _ := admissionFixture(t, false)
	j := admissionClaim(t, r)
	_, err := integrationDB.ExecContext(ctx, "DELETE FROM scheduler_outbox WHERE account_id=$1", a.ID)
	require.NoError(t, err)
	require.NoError(t, accounts.BindGroups(ctx, a.ID, nil))
	var events int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox WHERE account_id=$1 AND event_type=$2", a.ID, service.SchedulerOutboxEventAccountGroupsChanged).Scan(&events))
	require.Equal(t, 1, events, "empty replacement must invalidate scheduler membership")
	require.NoError(t, r.FinishAdmission(ctx, j, "passed", admissionSuccess(), admissionSuccess()))
	got, err := accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.False(t, got.Schedulable)
	require.Empty(t, got.GroupIDs)
}

func TestAdmissionMembershipContentionDoesNotDeadlock(t *testing.T) {
	ctx := context.Background()
	accounts, r, a, _, _ := admissionFixture(t, true)
	j := admissionClaim(t, r)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	var accountID int64
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT account_id FROM account_groups WHERE account_id=$1 FOR UPDATE", a.ID).Scan(&accountID))
	finishCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = r.FinishAdmission(finishCtx, j, "passed", admissionSuccess(), admissionSuccess())
	require.Error(t, err)
	require.NotErrorIs(t, err, context.DeadlineExceeded, "must release the account lock promptly")
	_, err = tx.ExecContext(ctx, "DELETE FROM account_groups WHERE account_id=$1", a.ID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	got, err := accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.False(t, got.Schedulable)
	require.Empty(t, got.GroupIDs)
}

func TestAdmissionExplicitAdministratorCanTakeOver(t *testing.T) {
	ctx := context.Background()
	accounts, r, a, target, _ := admissionFixture(t, false)
	old := admissionClaim(t, r)
	require.NoError(t, accounts.BindGroups(ctx, a.ID, []int64{target}))
	require.NoError(t, accounts.PauseAdmission(ctx, a.ID, true))
	got, err := accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, got.Schedulable)
	require.False(t, got.AdmissionBlocked())
	require.Equal(t, []int64{target}, got.GroupIDs)
	require.NoError(t, r.FinishAdmission(ctx, old, "failed", &service.ScheduledTestResult{Status: "failed", ErrorMessage: "answer_mismatch: expected 21"}, admissionSuccess()))
	got, err = accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, got.Schedulable)
	require.Equal(t, []int64{target}, got.GroupIDs)
}

func TestAdmissionStaleWholeUpdatePreservesPromotion(t *testing.T) {
	ctx := context.Background()
	accounts, r, a, target, _ := admissionFixture(t, false)
	stale, err := accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.False(t, stale.Schedulable)
	j := admissionClaim(t, r)
	require.NoError(t, r.FinishAdmission(ctx, j, "passed", admissionSuccess(), admissionSuccess()))
	notes := "updated after admission"
	stale.Notes = &notes
	require.NoError(t, accounts.Update(ctx, stale))
	got, err := accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, got.Schedulable)
	require.False(t, got.AdmissionBlocked())
	require.Equal(t, []int64{target}, got.GroupIDs)
	require.Equal(t, &notes, got.Notes)
	var active bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT active FROM account_admission_jobs WHERE account_id=$1", a.ID).Scan(&active))
	require.True(t, active, "ordinary stale updates must not pause recurring tests")
}
