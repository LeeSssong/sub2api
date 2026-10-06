package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Opt-in fresh, disposable local database only. Never use an existing application DB.
func TestFusionOnlineMigrationsLocal(t *testing.T) {
	dsn, base := os.Getenv("FUSION_TEST_PG_DSN"), os.Getenv("FUSION_TEST_BASE_MIGRATIONS")
	if dsn == "" || base == "" {
		t.Skip("requires disposable FUSION_TEST_PG_DSN and baseline migration directory")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var tables int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname='public'").Scan(&tables))
	require.Zero(t, tables, "refuse nonempty database")
	require.NoError(t, applyMigrationsFS(ctx, db, os.DirFS(base)))
	_, err = db.ExecContext(ctx, `CREATE TABLE fusion_old_columns AS SELECT table_name,column_name,data_type FROM information_schema.columns WHERE table_schema='public'`)
	require.NoError(t, err)
	// ALTER waits must remain short even without migrate-only DSN options.
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "LOCK TABLE accounts IN ACCESS EXCLUSIVE MODE")
	require.NoError(t, err)
	started := time.Now()
	err = ApplyMigrations(ctx, db)
	require.Error(t, err)
	require.Contains(t, err.Error(), "lock timeout")
	require.Less(t, time.Since(started), 2*time.Second)
	require.NoError(t, tx.Rollback())

	// A later lock failure leaves earlier additive receipts intact and is retryable.
	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "LOCK TABLE groups IN ACCESS EXCLUSIVE MODE")
	require.NoError(t, err)
	err = ApplyMigrations(ctx, db)
	require.ErrorContains(t, err, "lock timeout")
	require.NoError(t, tx.Rollback())
	var applied bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename='241_add_account_group_rate_multiplier.sql')`).Scan(&applied))
	require.True(t, applied)
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, ApplyMigrations(ctx, db))
	var missing int
	// The snapshot is an ordinary test table, independent of pooled connection ownership.
	err = db.QueryRowContext(ctx, `SELECT count(*) FROM fusion_old_columns o WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns c WHERE c.table_schema='public' AND c.table_name=o.table_name AND c.column_name=o.column_name AND c.data_type=o.data_type)`).Scan(&missing)
	require.NoError(t, err)
	require.Zero(t, missing, "old reader column contracts preserved")
	var valid bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT indisvalid FROM pg_index WHERE indexrelid='scheduled_test_quality_account_unique'::regclass`).Scan(&valid))
	require.True(t, valid)
	// Duplicate ownership fails closed; retry repairs an invalid concurrent index
	// only after the operator resolves duplicates (never deletes user rows).
	_, err = db.ExecContext(ctx, "DROP INDEX scheduled_test_quality_account_unique")
	require.NoError(t, err)
	var accountID int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type,credentials) VALUES ('fusion-test','openai','oauth','{}') RETURNING id`).Scan(&accountID))
	_, err = db.ExecContext(ctx, `INSERT INTO scheduled_test_plans(account_id,pelican_config) VALUES ($1,'{"quality":{}}'),($1,'{"quality":{}}')`, accountID)
	require.NoError(t, err)
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	err = prepareNonTransactionalMigration(ctx, conn, "252_account_quality_unique_notx.sql")
	require.ErrorContains(t, err, "duplicate quality rules")
	_, err = conn.ExecContext(ctx, `CREATE UNIQUE INDEX CONCURRENTLY scheduled_test_quality_account_unique ON scheduled_test_plans(account_id) WHERE pelican_config->'quality' IS NOT NULL`)
	require.Error(t, err)
	invalid, err := indexIsInvalid(ctx, conn, "scheduled_test_quality_account_unique")
	require.NoError(t, err)
	require.True(t, invalid)
	_, err = conn.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE id=(SELECT MAX(id) FROM scheduled_test_plans WHERE account_id=$1)`, accountID)
	require.NoError(t, err)
	require.NoError(t, prepareNonTransactionalMigration(ctx, conn, "252_account_quality_unique_notx.sql"))
	_, err = conn.ExecContext(ctx, `CREATE UNIQUE INDEX CONCURRENTLY scheduled_test_quality_account_unique ON scheduled_test_plans(account_id) WHERE pelican_config->'quality' IS NOT NULL`)
	require.NoError(t, err)

}
