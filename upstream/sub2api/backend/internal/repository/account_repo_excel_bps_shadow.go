package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

var _ service.AccountExcelBPSRecoveryRepository = (*accountRepository)(nil)

const excelBPSConfigSQL = `COALESCE((SELECT jsonb_object_agg(key,value) FROM jsonb_each(accounts.extra) WHERE key LIKE 'openai_excel_bps%' AND key NOT IN ('openai_excel_bps_recovery','openai_excel_bps_403_disabled_at','openai_excel_bps_403_moved_at','openai_excel_bps_403_moved_group_id','openai_excel_bps_last_disabled_status','openai_excel_bps_last_disabled_at')), '{}'::jsonb)`

func excelBPSConfigJSON(extra map[string]any) string {
	config := map[string]any{}
	for key, value := range extra {
		if strings.HasPrefix(key, "openai_excel_bps") && key != service.ExcelBPSRecoveryKey && key != service.ExcelBPS403DisabledAtKey && key != service.ExcelBPS403MovedAtKey && key != service.ExcelBPS403MovedGroupIDKey && key != "openai_excel_bps_last_disabled_status" && key != "openai_excel_bps_last_disabled_at" {
			config[key] = value
		}
	}
	raw, _ := json.Marshal(config)
	return string(raw)
}

const excelBPSNormalSQL = `deleted_at IS NULL AND parent_account_id IS NULL AND platform='openai' AND type='oauth'
 AND status='active' AND schedulable=true
 AND COALESCE(extra->'account_admission_blocked','false'::jsonb) <> 'true'::jsonb
 AND (NOT auto_pause_on_expired OR expires_at IS NULL OR expires_at>NOW())
 AND (overload_until IS NULL OR overload_until<=NOW())
 AND (rate_limit_reset_at IS NULL OR rate_limit_reset_at<=NOW())
 AND (temp_unschedulable_until IS NULL OR temp_unschedulable_until<=NOW())`

// Each transition and its cache invalidation event commit together.
func (r *accountRepository) excelBPSRecoveryWrite(ctx context.Context, write func(context.Context, *dbent.Client) ([]int64, error)) (bool, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	ids, err := write(dbent.NewTxContext(ctx, tx), tx.Client())
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if err = enqueueSchedulerOutbox(dbent.NewTxContext(ctx, tx), tx.Client(), service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
			return false, err
		}
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	for _, id := range ids {
		r.syncSchedulerAccountSnapshot(ctx, id)
	}
	return len(ids) > 0, nil
}
func (r *accountRepository) DegradeExcelBPS(ctx context.Context, a *service.Account, status int) (bool, error) {
	if a == nil || !a.IsExcelBPSEnabled() || !service.IsExcelBPSDegradationStatus(status) {
		return false, nil
	}
	if status == 403 && a.Extra[service.ExcelBPSShadowRecoveryKey] != true && a.Extra["openai_excel_bps_auto_disable_on_403"] == false {
		return false, nil
	}
	credentials, err := json.Marshal(a.Credentials)
	if err != nil {
		return false, err
	}
	extra := excelBPSConfigJSON(a.Extra)
	now := time.Now().UTC()
	state := service.ExcelBPSRecoveryState{Active: true, Generation: uuid.NewString(), DegradedAt: now, TriggerStatus: status, NextProbeAt: now.Add(service.ExcelBPSRecoveryDelay(0))}
	stateJSON, _ := json.Marshal(state)
	recoveryJSON, _ := json.Marshal(a.Extra[service.ExcelBPSRecoveryKey])
	return r.excelBPSRecoveryWrite(ctx, func(ctx context.Context, client *dbent.Client) ([]int64, error) {
		matches, err := lockExcelBPSProxyTransport(ctx, client, a)
		if err != nil || !matches {
			return nil, err
		}
		result, err := client.ExecContext(ctx, `UPDATE accounts SET extra=CASE WHEN extra->'openai_excel_bps_shadow_recovery'='true'::jsonb
   THEN extra || jsonb_build_object('openai_excel_bps_recovery',$4::jsonb)
   ELSE extra || jsonb_build_object('openai_excel_bps',false,'openai_excel_bps_last_disabled_status',$5::int,'openai_excel_bps_last_disabled_at',$6::text)
    || CASE WHEN $5::int=403 THEN jsonb_build_object('openai_excel_bps_403_disabled_at',$6::text) ELSE '{}'::jsonb END END,
   updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond')
   WHERE id=$1 AND deleted_at IS NULL AND parent_account_id IS NULL AND platform='openai' AND type='oauth'
    AND credentials=$2::jsonb AND `+excelBPSConfigSQL+`=$3::jsonb
    AND extra->'openai_excel_bps'='true'::jsonb
    AND COALESCE(extra->'openai_excel_bps_recovery'->'active','false'::jsonb)<>'true'::jsonb
 AND COALESCE(extra->'openai_excel_bps_recovery','null'::jsonb)=$7::jsonb AND proxy_id IS NOT DISTINCT FROM $8`, a.ID, string(credentials), string(extra), string(stateJSON), status, now.Format(time.RFC3339), string(recoveryJSON), a.ProxyID)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if n == 0 {
			return nil, err
		}
		return []int64{a.ID}, err
	})
}

func (r *accountRepository) ClaimDueExcelBPSRecoveries(ctx context.Context, now time.Time, limit int) ([]*service.Account, error) {
	if limit < 1 {
		return nil, nil
	}
	if limit > 3 {
		limit = 3
	}
	token := uuid.NewString()
	var ids []int64
	_, err := r.excelBPSRecoveryWrite(ctx, func(ctx context.Context, client *dbent.Client) ([]int64, error) {
		rows, err := client.QueryContext(ctx, `WITH due AS (SELECT id FROM accounts WHERE `+excelBPSNormalSQL+`
   AND extra->'openai_excel_bps'='true'::jsonb AND extra->'openai_excel_bps_shadow_recovery'='true'::jsonb
   AND extra->'openai_excel_bps_recovery'->'active'='true'::jsonb
   AND (extra->'openai_excel_bps_recovery'->>'next_probe_at')::timestamptz <= $1
   AND COALESCE((extra->'openai_excel_bps_recovery'->>'lease_until')::timestamptz,'epoch'::timestamptz)<=$1
   ORDER BY id FOR UPDATE SKIP LOCKED LIMIT $2)
   UPDATE accounts SET extra=jsonb_set(extra,'{openai_excel_bps_recovery}',(extra->'openai_excel_bps_recovery') || jsonb_build_object('lease_token',$3::text,'lease_until',$4::text)),
    updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond')
   WHERE id IN (SELECT id FROM due) RETURNING id`, now, limit, token, now.Add(4*time.Minute).UTC().Format(time.RFC3339Nano))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	})
	if err != nil {
		return nil, err
	}
	accounts := make([]*service.Account, 0, len(ids))
	for _, id := range ids {
		a, err := r.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if a.ExcelBPSRecovery().LeaseToken == token && a.IsSchedulableAt(now) {
			accounts = append(accounts, a)
		}
	}
	return accounts, nil
}

func (r *accountRepository) FinishExcelBPSRecovery(ctx context.Context, a *service.Account, passed bool, now time.Time) (bool, error) {
	state := a.ExcelBPSRecovery()
	if !state.Active || state.LeaseToken == "" {
		return false, nil
	}
	credentials, _ := json.Marshal(a.Credentials)
	extra := excelBPSConfigJSON(a.Extra)
	recovery, _ := json.Marshal(a.Extra[service.ExcelBPSRecoveryKey])
	state.LeaseToken = ""
	state.LeaseUntil = time.Time{}
	if passed {
		state.Active = false
		state.LastResult = "passed"
	} else {
		state.Failures++
		state.LastResult = "failed"
		state.NextProbeAt = now.Add(service.ExcelBPSRecoveryDelay(state.Failures))
	}
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return false, fmt.Errorf("marshal BPS recovery: %w", err)
	}
	return r.excelBPSRecoveryWrite(ctx, func(ctx context.Context, client *dbent.Client) ([]int64, error) {
		matches, err := lockExcelBPSProxyTransport(ctx, client, a)
		if err != nil || !matches {
			return nil, err
		}
		result, err := client.ExecContext(ctx, `UPDATE accounts SET extra=jsonb_set(extra,'{openai_excel_bps_recovery}',$4::jsonb),updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond')
   WHERE id=$1 AND credentials=$2::jsonb AND `+excelBPSConfigSQL+`=$3::jsonb AND extra->'openai_excel_bps_recovery'=$5::jsonb
    AND `+excelBPSNormalSQL+` AND extra->'openai_excel_bps'='true'::jsonb AND extra->'openai_excel_bps_shadow_recovery'='true'::jsonb
    AND (extra->'openai_excel_bps_recovery'->>'lease_until')::timestamptz > $6 AND proxy_id IS NOT DISTINCT FROM $7`, a.ID, string(credentials), string(extra), string(stateJSON), string(recovery), now, a.ProxyID)
		if err != nil {
			return nil, err
		}
		n, err := result.RowsAffected()
		if n == 0 {
			return nil, err
		}
		return []int64{a.ID}, err
	})
}

// Lock the referenced row before the account transition so a same-ID transport
// edit cannot commit between comparison and restore/degradation. Metadata-only
// proxy updates do not invalidate the snapshot.
func lockExcelBPSProxyTransport(ctx context.Context, client *dbent.Client, a *service.Account) (bool, error) {
	if a.ProxyID == nil {
		return true, nil
	}
	if a.Proxy == nil || a.Proxy.ID != *a.ProxyID {
		return false, nil
	}
	rows, err := client.QueryContext(ctx, `SELECT protocol,host,port,COALESCE(username,''),COALESCE(password,''),status,expires_at,fallback_mode,backup_proxy_id FROM proxies WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, *a.ProxyID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	current := service.Proxy{ID: *a.ProxyID}
	var expires sql.NullTime
	var backup sql.NullInt64
	if err = rows.Scan(&current.Protocol, &current.Host, &current.Port, &current.Username, &current.Password, &current.Status, &expires, &current.FallbackMode, &backup); err != nil {
		return false, err
	}
	if expires.Valid {
		current.ExpiresAt = &expires.Time
	}
	if backup.Valid {
		current.BackupProxyID = &backup.Int64
	}
	if err = rows.Err(); err != nil {
		return false, err
	}
	return service.ExcelBPSProxyTransportFingerprint(a.Proxy) == service.ExcelBPSProxyTransportFingerprint(&current), nil
}
