//go:build route_quality_integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageWritesWithoutQualitySnapshotsPostgres(t *testing.T) {
	dsn := os.Getenv("ROUTE_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("set ROUTE_QUALITY_TEST_DSN for isolated PostgreSQL verification")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	started := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	log := &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, GroupID: func() *int64 { v := int64(7); return &v }(), RequestID: "single", Model: "gpt-5", CreatedAt: started.Add(time.Minute)}
	prepared := prepareUsageLogInsert(log)
	query, _ := buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
	match := regexp.MustCompile(`(?s)INSERT INTO usage_logs \((.*?)\)`).FindStringSubmatch(query)
	require.Len(t, match, 2)
	require.NotContains(t, match[1], "quality_request_started_at")
	require.NotContains(t, match[1], "quality_status")
	columns := strings.Split(match[1], ",")
	require.Len(t, columns, len(usageLogInsertArgTypes))
	definitions := []string{"id BIGSERIAL PRIMARY KEY"}
	for i, column := range columns {
		definitions = append(definitions, strings.TrimSpace(column)+" "+usageLogInsertArgTypes[i])
	}
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("retained_snapshot_schema=%v", legacy), func(t *testing.T) {
			_, err = db.ExecContext(ctx, "DROP TABLE IF EXISTS pg_temp.usage_logs; CREATE TEMP TABLE usage_logs("+strings.Join(definitions, ",")+",UNIQUE(request_id,api_key_id));")
			require.NoError(t, err)
			if legacy {
				_, err = db.ExecContext(ctx, `ALTER TABLE usage_logs ADD COLUMN quality_request_started_at timestamptz, ADD COLUMN quality_status text;
CREATE OR REPLACE FUNCTION pg_temp.snapshot_usage_quality_status() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.quality_request_started_at IS NULL THEN
  NEW.quality_status := NULL;
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'removed snapshot path was invoked';
END $$;
CREATE TRIGGER usage_quality_snapshot BEFORE INSERT ON usage_logs FOR EACH ROW EXECUTE FUNCTION pg_temp.snapshot_usage_quality_status();`)
				require.NoError(t, err)
			}
			log.RequestID = "single"
			repo := &usageLogRepository{sql: db}
			inserted, err := repo.createSingle(ctx, db, log)
			require.NoError(t, err)
			require.True(t, inserted)
			log.RequestID = "no-result"
			require.NoError(t, execUsageLogInsertNoResult(ctx, db, prepareUsageLogInsert(log)))
			log.RequestID = "batch"
			prepared = prepareUsageLogInsert(log)
			key := usageLogBatchKey(log.RequestID, log.APIKeyID)
			query, args := buildUsageLogBatchInsertQuery([]string{key}, map[string]usageLogInsertPrepared{key: prepared})
			rows, err := db.QueryContext(ctx, query, args...)
			require.NoError(t, err)
			require.NoError(t, rows.Close())
			log.RequestID = "best-effort"
			prepared = prepareUsageLogInsert(log)
			query, args = buildUsageLogBestEffortInsertQuery([]usageLogInsertPrepared{prepared})
			rows, err = db.QueryContext(ctx, query, args...)
			require.NoError(t, err)
			require.NoError(t, rows.Close())
			var count int
			require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs`).Scan(&count))
			require.Equal(t, 4, count, "all four usage insert paths work without snapshot data")
			log.RequestID = "single"
			inserted, err = repo.createSingle(ctx, db, log)
			require.NoError(t, err)
			require.False(t, inserted, "idempotent retry does not double write")
			if legacy {
				require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs WHERE quality_request_started_at IS NOT NULL OR quality_status IS NOT NULL`).Scan(&count))
				require.Zero(t, count, "retained trigger never records a snapshot")
			}
		})
	}
}
