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

// Temporary tables on one connection keep this fixture isolated from application data.
func TestRouteCacheTimelineMatchesAdminStreamPostgres(t *testing.T) {
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
 CREATE TEMP TABLE usage_logs(id bigserial,quality_status text,group_id bigint,account_id bigint,created_at timestamptz,usage_completeness text,first_token_ms double precision,duration_ms double precision,input_tokens bigint,output_tokens bigint,cache_creation_tokens bigint,cache_read_tokens bigint,logical_request_id text,request_id text,request_type smallint,stream boolean,openai_ws_mode boolean,total_cost numeric DEFAULT 0,actual_cost numeric DEFAULT 0);
 CREATE TEMP TABLE ops_error_logs(id bigserial,group_id bigint,account_id bigint,created_at timestamptz,status_code int,is_count_tokens boolean DEFAULT false,is_business_limited boolean DEFAULT false,error_owner text,error_phase text,error_source text,error_type text,error_message text,error_body text,upstream_error_message text,upstream_error_detail text,request_id text,client_request_id text);
 CREATE TEMP TABLE quality_rule_template_accounts(plan_id bigint,tested_group_id bigint);
 CREATE TEMP TABLE scheduled_test_results(id bigint,plan_id bigint,quality_round_id text,finished_at timestamptz,status text,error_message text,pelican_config jsonb,quality_judgment jsonb,response_text text);`)
	require.NoError(t, err)
	start := time.Date(2026, 10, 9, 11, 47, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	insert := func(group int64, at time.Time, key string, completeness any, requestType service.RequestType, stream, ws bool, input, creation, read int) {
		t.Helper()
		_, err := db.ExecContext(ctx, `INSERT INTO usage_logs(group_id,account_id,created_at,logical_request_id,request_id,usage_completeness,first_token_ms,duration_ms,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,request_type,stream,openai_ws_mode) VALUES($1,1,$2,$3,$3,$4,400,1000,$5,999999,$6,$7,$8,$9,$10)`, group, at, key, completeness, input, creation, read, int16(requestType), stream, ws)
		require.NoError(t, err)
	}
	// Duplicate logical/request IDs must contribute both rows. Partial/unknown
	// usage, cache creation and legacy stream flags must match the admin query.
	insert(7, start, "retry", "complete", service.RequestTypeStream, true, false, 10, 20, 70)
	insert(7, start.Add(time.Minute), "retry", "complete", service.RequestTypeStream, true, false, 80, 20, 0)
	insert(7, start.Add(2*time.Minute), "partial", "partial", service.RequestTypeStream, true, false, 0, 20, 80)
	insert(7, start.Add(3*time.Minute), "unknown", "unknown", service.RequestTypeStream, true, false, 0, 0, 200)
	insert(7, start.Add(4*time.Minute), "legacy", nil, service.RequestTypeUnknown, true, false, 60, 0, 40)
	insert(7, start.Add(6*time.Minute), "explicit", "complete", service.RequestTypeStream, false, true, 90, 0, 10)
	insert(7, start.Add(7*time.Minute), "client-error", "complete", service.RequestTypeStream, true, false, 50, 0, 50)
	_, err = db.ExecContext(ctx, `INSERT INTO ops_error_logs(group_id,account_id,created_at,status_code,error_owner,error_phase,error_type,request_id) VALUES(7,1,$1,400,'client','request','invalid_request','client-error')`, start.Add(7*time.Minute))
	require.NoError(t, err)
	// Explicit non-stream/WS and legacy WS or sync records must not contribute.
	insert(7, start.Add(time.Minute), "sync", "complete", service.RequestTypeSync, true, false, 10000, 0, 0)
	insert(7, start.Add(time.Minute), "ws", "complete", service.RequestTypeWSV2, true, true, 0, 0, 10000)
	insert(7, start.Add(time.Minute), "legacy-ws", "complete", service.RequestTypeUnknown, true, true, 10000, 0, 0)
	insert(7, start.Add(time.Minute), "legacy-sync", "complete", service.RequestTypeUnknown, false, false, 0, 0, 10000)
	insert(8, start.Add(time.Minute), "other-group", "complete", service.RequestTypeStream, true, false, 90, 0, 10)
	insert(10, start.Add(time.Minute), "zero", "unknown", service.RequestTypeStream, true, false, 0, 0, 0)
	insert(7, start.Add(-time.Minute), "before-start", "complete", service.RequestTypeStream, true, false, 10000, 0, 0)
	insert(7, end, "at-end", "complete", service.RequestTypeStream, true, false, 0, 0, 10000)

	monitor := &accountMonitorRepository{db: db}
	admin := &usageLogRepository{sql: db}
	streamType := int16(service.RequestTypeStream)
	assertBucket := func(t *testing.T, group int64, from, to time.Time, rate *float64) {
		t.Helper()
		trend, err := admin.GetUsageTrendWithFilters(ctx, from, to, "hour", 0, 0, 0, group, "", &streamType, nil, nil)
		require.NoError(t, err)
		var input, creation, read int64
		for _, point := range trend {
			input += point.InputTokens
			creation += point.CacheCreationTokens
			read += point.CacheReadTokens
		}
		denominator := input + creation + read
		if denominator == 0 {
			require.Nil(t, rate, "zero/no prompt tokens remain a no-sample value, displayed as zero")
			return
		}
		require.NotNil(t, rate)
		require.InDelta(t, float64(read)/float64(denominator), *rate, 1e-12, "same group/time bucket as admin stream trend")
	}
	for _, tc := range []struct {
		name         string
		window, step time.Duration
		buckets      int
	}{{"1h-5m", time.Hour, 5 * time.Minute, 12}, {"24h-hour", 24 * time.Hour, time.Hour, 24}, {"24h-day", 24 * time.Hour, 24 * time.Hour, 1}, {"7d-hour", 7 * 24 * time.Hour, time.Hour, 168}, {"7d-day", 7 * 24 * time.Hour, 24 * time.Hour, 7}} {
		t.Run(tc.name, func(t *testing.T) {
			points, err := monitor.ReadMonitorV4Timeline(ctx, []int64{7, 8, 9, 10}, end.Add(-tc.window), end, tc.step)
			require.NoError(t, err)
			require.Len(t, points, 4*tc.buckets)
			for _, point := range points {
				assertBucket(t, point.GroupID, point.Start, point.End, point.CacheHitRate)
			}
		})
	}
	points, err := monitor.ReadMonitorV4Timeline(ctx, []int64{7}, start, end, 5*time.Minute)
	require.NoError(t, err)
	require.InDelta(t, 390.0/600, *points[0].CacheHitRate, 1e-12)
	require.InDelta(t, 60.0/200, *points[1].CacheHitRate, 1e-12)
	require.Equal(t, 400.0, *points[0].TTFTP50MS, "cache changes must not change TTFT filtering")
	require.Equal(t, 9, points[0].SuccessCount, "SLA still counts all usage rows")
	require.Equal(t, 3, points[1].RequestCount, "SLA still includes the client error row")
	// Historic exclusions used for real metrics must not leak into raw cache usage.
	_, err = db.ExecContext(ctx, `TRUNCATE usage_logs,ops_error_logs`)
	require.NoError(t, err)
	historicalStart := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	insert(7, historicalStart, "historical", "unknown", service.RequestTypeStream, true, false, 25, 0, 75)
	historical, err := monitor.ReadMonitorV4Timeline(ctx, []int64{7}, historicalStart, historicalStart.Add(time.Hour), time.Hour)
	require.NoError(t, err)
	assertBucket(t, 7, historical[0].Start, historical[0].End, historical[0].CacheHitRate)
	require.InDelta(t, 0.75, *historical[0].CacheHitRate, 1e-12)
	require.Nil(t, historical[0].TTFTP50MS)
}
