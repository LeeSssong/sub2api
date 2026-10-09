package repository

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// This release expands the old schema while old API readers remain available.
// The caller provides only an empty disposable local database and a Git baseline.
func TestOctober09OfficialOnlineMigrationCompatibility(t *testing.T) {
	dsn, base := os.Getenv("OCTOBER09_TEST_PG_DSN"), os.Getenv("OCTOBER09_TEST_BASE")
	if dsn == "" || base == "" {
		t.Skip("requires fresh local October09 database and baseline")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname())
	require.True(t, strings.HasPrefix(strings.TrimPrefix(u.Path, "/"), "sub2api_oct09_"))
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname='public'`).Scan(&count))
	require.Zero(t, count, "refuse a populated database")
	require.NoError(t, applyMigrationsFS(ctx, db, os.DirFS(base)))
	_, err = db.ExecContext(ctx, `CREATE TABLE october09_old_receipts AS SELECT filename,checksum FROM schema_migrations;
 CREATE TABLE october09_old_columns AS SELECT table_name,column_name,data_type FROM information_schema.columns WHERE table_schema='public';
 INSERT INTO accounts(id,name,platform,type,credentials) VALUES(900001,'safe-fixture','openai','oauth','{}'),(900002,'safe-fixture','openai','oauth','{}');
 INSERT INTO account_ops_alerts(account_id,kind,account_name,signal,http_status,first_seen,last_seen,state,last_sent_at)
 VALUES(900001,'balance_low','safe-fixture','balance',0,NOW(),NOW(),'sent',NOW());`)
	require.NoError(t, err)
	// Trigger installation must fail quickly rather than queue behind live writes.
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `LOCK TABLE accounts IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	start := time.Now()
	err = ApplyMigrations(ctx, db)
	require.ErrorContains(t, err, "lock timeout")
	require.Less(t, time.Since(start), 2*time.Second)
	require.NoError(t, tx.Rollback())
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, VerifyMigrations(ctx, db))
	require.NoError(t, verifyMigrationsFS(ctx, db, os.DirFS(base)), "old binary verifies existing receipts after expansion")
	require.NoError(t, ApplyMigrations(ctx, db), "retry is idempotent")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM october09_old_receipts o WHERE NOT EXISTS(SELECT 1 FROM schema_migrations s WHERE s.filename=o.filename AND s.checksum=o.checksum)`).Scan(&count))
	require.Zero(t, count, "existing migration receipt checksums must survive")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM october09_old_columns o WHERE NOT EXISTS(SELECT 1 FROM information_schema.columns c WHERE c.table_schema='public' AND c.table_name=o.table_name AND c.column_name=o.column_name AND c.data_type=o.data_type)`).Scan(&count))
	require.Zero(t, count, "old reader columns must remain")
	var state string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT state FROM account_ops_alerts WHERE account_id=900001 AND kind='balance_low'`).Scan(&state))
	require.Equal(t, "sent", state)
	// Old worker's insert and update contracts remain usable after the extension.
	_, err = db.ExecContext(ctx, `INSERT INTO account_ops_alerts(account_id,kind,account_name,signal,http_status,first_seen,last_seen,state) VALUES(900002,'weekly_quota','safe-fixture','quota',0,NOW(),NOW(),'pending'); UPDATE account_ops_alerts SET state='sent',last_sent_at=NOW() WHERE account_id=900002;`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM account_ops_threshold_events`).Scan(&count))
	require.Zero(t, count, "empty old threshold queue transfers no tasks")
}
