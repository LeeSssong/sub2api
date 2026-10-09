//go:build route_quality_integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Dedicated local QA database; TEMP tables shadow application tables on a
// single connection, so this test cannot modify application data.
func TestRouteSLATimelinePostgres(t *testing.T) {
	dsn := os.Getenv("ROUTE_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ROUTE_QUALITY_TEST_DSN for isolated PostgreSQL verification")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
 CREATE TEMP TABLE usage_logs(id bigserial,group_id bigint,account_id bigint,created_at timestamptz,usage_completeness text,first_token_ms double precision,duration_ms double precision,input_tokens bigint,output_tokens bigint,cache_creation_tokens bigint,cache_read_tokens bigint,logical_request_id text,request_id text,request_type smallint DEFAULT 2,stream boolean DEFAULT true,openai_ws_mode boolean DEFAULT false);
 CREATE TEMP TABLE ops_error_logs(id bigserial,group_id bigint,account_id bigint,created_at timestamptz,status_code int,is_count_tokens boolean DEFAULT false,is_business_limited boolean DEFAULT false,error_owner text,error_phase text,error_source text,error_type text,error_message text,error_body text,upstream_error_message text,upstream_error_detail text,upstream_status_code int,request_id text,client_request_id text);
 CREATE TEMP TABLE quality_rule_template_accounts(plan_id bigint,tested_group_id bigint);
 CREATE TEMP TABLE scheduled_test_results(id bigint,plan_id bigint,quality_round_id text,finished_at timestamptz,status text,error_message text,pelican_config jsonb,quality_judgment jsonb,response_text text);`)
	require.NoError(t, err)
	end := time.Date(2026, 10, 9, 12, 47, 0, 0, time.UTC)
	start := end.Add(-time.Hour)
	usage := func(group int64, at time.Time, key, completeness string, first, input, creation, read int) {
		t.Helper()
		_, err := db.ExecContext(ctx, `INSERT INTO usage_logs(group_id,account_id,created_at,logical_request_id,request_id,usage_completeness,first_token_ms,duration_ms,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens) VALUES($1,1,$2,$3,$3,$4,$5,1000,$6,10,$7,$8)`, group, at, key, completeness, first, input, creation, read)
		require.NoError(t, err)
	}
	usage(7, start, "retry", "complete", 200, 100, 0, 100)
	usage(7, start.Add(time.Minute), "retry", "complete", 400, 100, 0, 100)
	usage(7, start.Add(6*time.Minute), "other", "complete", 600, 100, 0, 0)
	usage(7, start.Add(2*time.Minute), "partial", "partial", 9000, 1, 0, 0)
	usage(7, start.Add(3*time.Minute), "unknown", "unknown", 9000, 1, 0, 0)
	usage(8, start.Add(time.Minute), "retry", "complete", 1000, 100, 0, 100)
	// End is excluded; start is included. Out-of-window requests never leak in.
	usage(7, end, "at-end", "complete", 9000, 100, 0, 100)
	usage(7, end.Add(time.Minute), "after-end", "complete", 9000, 100, 0, 100)
	insertError := func(group int64, status int, owner, kind, message, key string, limited, countTokens bool) {
		t.Helper()
		_, err := db.ExecContext(ctx, `INSERT INTO ops_error_logs(group_id,account_id,created_at,status_code,error_owner,error_phase,error_type,error_message,request_id,is_business_limited,is_count_tokens) VALUES($1,1,$2,$3,$4,'request',$5,$6,$7,$8,$9)`, group, start.Add(4*time.Minute), status, owner, kind, message, key, limited, countTokens)
		require.NoError(t, err)
	}
	insertError(7, 502, "provider", "api_error", "failed", "same-error", false, false)
	insertError(7, 502, "provider", "api_error", "failed again", "same-error", false, false)
	insertError(7, 400, "client", "invalid_request", "bad request", "invalid", false, false)
	insertError(7, 400, "provider", "api_error", "model not supported", "unsupported", false, false)
	insertError(7, 429, "provider", "api_error", "rate limit", "429", false, false)
	insertError(7, 529, "provider", "api_error", "overloaded", "529", false, false)
	insertError(7, 402, "provider", "balance", "balance exhausted", "balance", true, false)
	insertError(7, 499, "client", "client_canceled", "canceled", "canceled", false, false)
	insertError(7, 502, "provider", "api_error", "count tokens", "count", false, true)
	insertError(7, 200, "provider", "api_error", "recovered", "retry", false, false)
	insertError(8, 502, "provider", "api_error", "other group", "same-error", false, false)
	repo := &accountMonitorRepository{db: db}
	ops := &opsRepository{db: db}
	for _, tc := range []struct {
		name         string
		window, step time.Duration
		buckets      int
	}{{"1h-5m", time.Hour, 5 * time.Minute, 12}, {"24h-hour", 24 * time.Hour, time.Hour, 24}, {"24h-day", 24 * time.Hour, 24 * time.Hour, 1}, {"7d-hour", 7 * 24 * time.Hour, time.Hour, 168}, {"7d-day", 7 * 24 * time.Hour, 24 * time.Hour, 7}} {
		t.Run(tc.name, func(t *testing.T) {
			points, err := repo.ReadMonitorV4Timeline(ctx, []int64{7, 8, 9}, end.Add(-tc.window), end, tc.step)
			require.NoError(t, err)
			require.Len(t, points, 3*tc.buckets)
			totals := map[int64][2]int{}
			for _, p := range points {
				group := p.GroupID
				filter := &service.OpsDashboardFilter{GroupID: &group}
				success, _, err := ops.queryUsageCounts(ctx, filter, p.Start, p.End)
				require.NoError(t, err)
				_, _, errorsSLA, _, _, _, err := ops.queryErrorCounts(ctx, filter, p.Start, p.End)
				require.NoError(t, err)
				require.EqualValues(t, success, p.SuccessCount, "same group and bucket as admin SLA")
				require.EqualValues(t, success+errorsSLA, p.RequestCount)
				total := totals[group]
				totals[group] = [2]int{total[0] + p.SuccessCount, total[1] + p.RequestCount}
				if group == 9 {
					require.Nil(t, p.CacheHitRate)
					require.Nil(t, p.TTFTP50MS)
				}
			}
			require.Equal(t, [2]int{5, 11}, totals[7])
			require.Equal(t, [2]int{1, 2}, totals[8])
			require.Equal(t, [2]int{0, 0}, totals[9])
		})
	}
	points, err := repo.ReadMonitorV4Timeline(ctx, []int64{7}, start, end, 5*time.Minute)
	require.NoError(t, err)
	// Cache uses all four stream usage rows (partial/unknown/duplicate included),
	// while P50 continues to use successful deduplicated real events.
	require.InDelta(t, 200.0/402, *points[0].CacheHitRate, 1e-9)
	require.Equal(t, 400.0, *points[0].TTFTP50MS)
	require.Equal(t, 600.0, *points[1].TTFTP50MS)
	require.Equal(t, 0.0, *points[1].CacheHitRate)
	_, err = db.ExecContext(ctx, `TRUNCATE usage_logs,ops_error_logs`)
	require.NoError(t, err)
	historicStart := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	usage(7, historicStart, "historical", "complete", 200, 100, 0, 100)
	historical, err := repo.ReadMonitorV4Timeline(ctx, []int64{7}, historicStart, historicStart.Add(time.Hour), time.Hour)
	require.NoError(t, err)
	require.Equal(t, 1, historical[0].SuccessCount, "SLA does not apply real metric historical exclusions")
	require.InDelta(t, 0.5, *historical[0].CacheHitRate, 1e-9, "cache follows admin usage without historical exclusions")
	require.Nil(t, historical[0].TTFTP50MS)
}
