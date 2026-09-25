package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// CreateWithAdmission commits the isolated account, temporary membership, durable
// job and scheduler outbox together. The formal groups are only a job snapshot.
func (r *accountRepository) CreateWithAdmission(ctx context.Context, a *service.Account, targets []int64, temporary *int64) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	groups := append([]int64(nil), targets...)
	if temporary != nil {
		groups = append(groups, *temporary)
	}
	if err = validateAdmissionGroupsSQL(ctx, client, a.Platform, a.IsMixedSchedulingEnabled(), groups); err != nil {
		return err
	}
	credentials, err := json.Marshal(a.Credentials)
	if err != nil {
		return err
	}
	// Platform/type serialization also covers differently shaped JSON identities.
	if _, err = client.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "account-admission:"+a.Platform+":"+a.Type); err != nil {
		return err
	}
	rows, err := client.QueryContext(ctx, `SELECT id FROM accounts WHERE deleted_at IS NULL AND platform=$1 AND type=$2 AND (credentials=$3::jsonb
 OR (COALESCE($3::jsonb->>'api_key','')<>'' AND credentials->>'api_key'=$3::jsonb->>'api_key' AND COALESCE(credentials->>'base_url','')=COALESCE($3::jsonb->>'base_url',''))
 OR (COALESCE($3::jsonb->>'chatgpt_account_id','')<>'' AND credentials->>'chatgpt_account_id'=$3::jsonb->>'chatgpt_account_id' AND COALESCE(credentials->>'chatgpt_user_id','')=COALESCE($3::jsonb->>'chatgpt_user_id',''))
 OR (COALESCE($3::jsonb->>'email','')<>'' AND lower(credentials->>'email')=lower($3::jsonb->>'email'))
 OR (COALESCE($3::jsonb->>'refresh_token','')<>'' AND credentials->>'refresh_token'=$3::jsonb->>'refresh_token')
 OR (COALESCE($3::jsonb->>'access_token','')<>'' AND credentials->>'access_token'=$3::jsonb->>'access_token')) LIMIT 1`, a.Platform, a.Type, string(credentials))
	if err != nil {
		return err
	}
	duplicate := rows.Next()
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if duplicate {
		return service.ErrAdmissionDuplicate
	}
	a.Schedulable = false
	if a.Extra == nil {
		a.Extra = map[string]any{}
	}
	a.Extra[service.AccountAdmissionBlockedKey] = true
	if err = createAccountRecord(ctx, client, a); err != nil {
		return err
	}
	a.GroupIDs = nil
	if temporary != nil {
		if _, err = client.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id,priority) VALUES($1,$2,1)`, a.ID, *temporary); err != nil {
			return err
		}
		a.GroupIDs = []int64{*temporary}
	}
	if _, err = client.ExecContext(ctx, `INSERT INTO account_admission_jobs(account_id,target_group_ids,test_group_id) VALUES($1,$2,$3)`, a.ID, pq.Array(targets), temporary); err != nil {
		return err
	}
	if err = enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &a.ID, nil, buildSchedulerGroupPayload(a.GroupIDs)); err != nil {
		return err
	}
	return tx.Commit()
}

func validateAdmissionGroupsSQL(ctx context.Context, exec sqlExecutor, platform string, mixed bool, ids []int64) error {
	if err := lockLiveGroups(ctx, exec, ids); err != nil {
		return err
	}
	rows, err := exec.QueryContext(ctx, `SELECT id FROM groups WHERE id=ANY($1) AND status='active' AND (platform=$2 OR platform='composite' OR ($3 AND platform IN ('anthropic','gemini'))) AND deleted_at IS NULL`, pq.Array(ids), platform, mixed)
	if err != nil {
		return err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if n != len(ids) {
		return errors.New("admission groups must be active, compatible and unique")
	}
	return nil
}

type accountAdmissionRepository struct {
	db       *sql.DB
	accounts service.AccountRepository
}

func NewAccountAdmissionRepository(db *sql.DB, accounts service.AccountRepository) service.AccountAdmissionRepository {
	return &accountAdmissionRepository{db: db, accounts: accounts}
}
func (r *accountAdmissionRepository) ClaimAdmission(ctx context.Context) (*service.AccountAdmissionJob, error) {
	job := &service.AccountAdmissionJob{LeaseToken: uuid.NewString()}
	// Reclaim an expired lease after process death, preserving one job per account.
	err := r.db.QueryRowContext(ctx, `WITH candidate AS (
 SELECT j.account_id FROM account_admission_jobs j JOIN accounts a ON a.id=j.account_id
 WHERE j.active AND a.deleted_at IS NULL AND j.next_run_at<=NOW() AND (j.lease_until IS NULL OR j.lease_until<NOW())
 ORDER BY j.next_run_at,j.account_id FOR UPDATE OF j SKIP LOCKED LIMIT 1)
 UPDATE account_admission_jobs j SET lease_token=$1,lease_until=NOW()+interval '10 minutes',
 credentials_signature=account_admission_credential_signature(a.credentials,a.platform,a.type,a.proxy_id),
 groups_snapshot=COALESCE((SELECT jsonb_agg(jsonb_build_object('group_id',ag.group_id,'priority',ag.priority,'allowed_models',ag.allowed_models) ORDER BY ag.group_id) FROM account_groups ag WHERE ag.account_id=j.account_id),'[]'::jsonb),
 schedulable_snapshot=a.schedulable,updated_at=NOW() FROM candidate c,accounts a
 WHERE j.account_id=c.account_id AND a.id=j.account_id RETURNING j.account_id,j.generation,j.lease_until`, job.LeaseToken).Scan(&job.AccountID, &job.Generation, &job.LeaseUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (r *accountAdmissionRepository) FinishAdmission(ctx context.Context, job *service.AccountAdmissionJob, outcome string, candy, pelican *service.ScheduledTestResult) error {
	outcome = service.AccountAdmissionOutcome(candy, pelican)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Group deletion locks group rows before touching memberships. Match that
	// order before taking the account lock, including the temporary group.
	groupLocks, lockErr := tx.QueryContext(ctx, "SELECT g.id FROM groups g JOIN account_admission_jobs j ON g.id=ANY(j.target_group_ids) OR g.id=j.test_group_id WHERE j.account_id=$1 ORDER BY g.id FOR SHARE OF g", job.AccountID)
	if lockErr != nil {
		return lockErr
	}
	for groupLocks.Next() {
	}
	lockErr = groupLocks.Err()
	groupLocks.Close()
	if lockErr != nil {
		return lockErr
	}
	var platform, signature, status string
	var schedulable, eligible, mixed bool
	err = tx.QueryRowContext(ctx, `SELECT platform,account_admission_credential_signature(credentials,platform,type,proxy_id),status,schedulable,
 (expires_at IS NULL OR expires_at>NOW()),(platform='antigravity' AND extra->'mixed_scheduling'='true'::jsonb) IS TRUE FROM accounts WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, job.AccountID).Scan(&platform, &signature, &status, &schedulable, &eligible, &mixed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var valid, blocked, expectedSchedulable bool
	var targets pq.Int64Array
	var temporary sql.NullInt64
	var expectedSignature string
	var expectedGroups, currentGroups []byte
	err = tx.QueryRowContext(ctx, `SELECT active AND generation=$2 AND lease_token=$3 AND lease_until>NOW(),blocked,target_group_ids,test_group_id,credentials_signature,groups_snapshot,schedulable_snapshot FROM account_admission_jobs WHERE account_id=$1 FOR UPDATE`, job.AccountID, job.Generation, job.LeaseToken).Scan(&valid, &blocked, &targets, &temporary, &expectedSignature, &expectedGroups, &expectedSchedulable)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !valid {
		return nil
	}
	// Legacy membership writers may already hold a tuple while their trigger
	// waits for this account. NOWAIT releases our transaction instead of forming
	// a deadlock, allowing the operator to finish and invalidate the lease.
	locked, lockErr := tx.QueryContext(ctx, "SELECT account_id FROM account_groups WHERE account_id=$1 ORDER BY group_id FOR UPDATE NOWAIT", job.AccountID)
	if lockErr != nil {
		return lockErr
	}
	for locked.Next() {
	}
	lockErr = locked.Err()
	locked.Close()
	if lockErr != nil {
		return lockErr
	}
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(jsonb_build_object('group_id',group_id,'priority',priority,'allowed_models',allowed_models) ORDER BY group_id),'[]'::jsonb) FROM account_groups WHERE account_id=$1`, job.AccountID).Scan(&currentGroups)
	if err != nil {
		return err
	}
	var before, after any
	if err = json.Unmarshal(expectedGroups, &before); err != nil {
		return err
	}
	if err = json.Unmarshal(currentGroups, &after); err != nil {
		return err
	}
	if signature != expectedSignature || schedulable != expectedSchedulable || !reflect.DeepEqual(before, after) {
		outcome = "inconclusive"
	}
	candyJSON, err := json.Marshal(candy)
	if err != nil {
		return err
	}
	pelicanJSON, err := json.Marshal(pelican)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `SELECT set_config('sub2api.admission_worker','on',true)`); err != nil {
		return err
	}
	state := "pending"
	if !blocked {
		state = "admitted"
	}
	changed := false
	if outcome == "passed" && status == service.StatusActive && eligible {
		if err = validateAdmissionGroupsSQL(ctx, tx, platform, mixed, targets); err != nil {
			outcome = "inconclusive"
		} else if blocked {
			if temporary.Valid {
				if _, err = tx.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id=$1 AND group_id=$2`, job.AccountID, temporary.Int64); err != nil {
					return err
				}
			}
			for i, id := range targets {
				if _, err = tx.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id,priority) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, job.AccountID, id, i+1); err != nil {
					return err
				}
			}
			blocked = false
			state = "admitted"
			changed = true
		}
	}
	if outcome == "failed" {
		state = "quarantined"
		if !blocked {
			if _, err = tx.ExecContext(ctx, `DELETE FROM account_groups WHERE account_id=$1 AND group_id=ANY($2)`, job.AccountID, pq.Array(targets)); err != nil {
				return err
			}
			blocked = true
			changed = true
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE account_admission_jobs SET state=$2,blocked=$3,last_outcome=$4,candy_result=$5,pelican_result=$6,last_run_at=NOW(),next_run_at=NOW()+interval '10 minutes',lease_token=NULL,lease_until=NULL,updated_at=NOW() WHERE account_id=$1`, job.AccountID, state, blocked, outcome, string(candyJSON), string(pelicanJSON)); err != nil {
		return err
	}
	if changed {
		if _, err = tx.ExecContext(ctx, `UPDATE accounts SET schedulable=$2,extra=jsonb_set(COALESCE(extra,'{}'::jsonb),'{account_admission_blocked}',to_jsonb($3::boolean),true),updated_at=clock_timestamp() WHERE id=$1`, job.AccountID, !blocked, blocked); err != nil {
			return err
		}
		groups := append([]int64(nil), targets...)
		if temporary.Valid {
			groups = append(groups, temporary.Int64)
		}
		if err = enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountGroupsChanged, &job.AccountID, nil, buildSchedulerGroupPayload(groups)); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if changed {
		if native, ok := r.accounts.(*accountRepository); ok {
			native.syncSchedulerAccountSnapshot(ctx, job.AccountID)
		}
	}
	return nil
}

func (r *accountRepository) PauseAdmission(ctx context.Context, id int64, enabling bool) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	client := tx.Client()
	rows, err := client.QueryContext(ctx, `SELECT a.id,j.blocked FROM accounts a LEFT JOIN account_admission_jobs j ON j.account_id=a.id WHERE a.id=$1 FOR UPDATE OF a`, id)
	if err != nil {
		return err
	}
	var aid int64
	var blocked sql.NullBool
	if rows.Next() {
		err = rows.Scan(&aid, &blocked)
	}
	rowErr := rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if rowErr != nil {
		return rowErr
	}
	if !blocked.Valid {
		return tx.Commit()
	}
	if _, err = client.ExecContext(ctx, "SELECT set_config('sub2api.admission_manual_scheduling','on',true)"); err != nil {
		return err
	}
	if _, err = client.ExecContext(ctx, "UPDATE accounts SET schedulable=$2,updated_at=NOW() WHERE id=$1", id, enabling); err != nil {
		return err
	}
	if err = enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	r.syncSchedulerAccountSnapshot(ctx, id)
	return nil
}

// Caller owns an account transaction. Empty replacement is still an explicit
// operator action and must cancel the task even when there are no tuples.
func pauseAdmissionForGroupEdit(ctx context.Context, exec sqlExecutor, id int64) error {
	rows, err := exec.QueryContext(ctx, "SELECT id FROM accounts WHERE id=$1 FOR UPDATE", id)
	if err != nil {
		return err
	}
	for rows.Next() {
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, "UPDATE account_admission_jobs SET active=false,state='paused',generation=generation+1,lease_token=NULL,lease_until=NULL,updated_at=NOW() WHERE account_id=$1 AND active", id)
	return err
}
