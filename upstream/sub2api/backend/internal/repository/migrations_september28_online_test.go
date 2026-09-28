package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Explicitly opt in to a fresh local disposable PostgreSQL database. Baseline
// SQL must be exported from the deployed root HEAD, not the integration tree.
// This test has no dependency on the package-wide integrationDB/testcontainer.
func TestSeptember28OnlineMigrationsLocal(t *testing.T) {
	dsn := os.Getenv("SEPTEMBER28_TEST_PG_DSN")
	base := os.Getenv("SEPTEMBER28_TEST_BASE_MIGRATIONS")
	if dsn == "" || base == "" {
		t.Skip("requires SEPTEMBER28_TEST_PG_DSN and SEPTEMBER28_TEST_BASE_MIGRATIONS")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"postgres", "postgresql"}, u.Scheme, "use an explicit PostgreSQL URL")
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname(), "local disposable database only")
	require.True(t, strings.HasPrefix(strings.TrimPrefix(u.Path, "/"), "sub2api_sep28_"), "database name must start with sub2api_sep28_")
	for key := range u.Query() {
		require.Contains(t, []string{"sslmode", "connect_timeout"}, key, "refuse connection-routing overrides or unreviewed URL options")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	baselineDB, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer baselineDB.Close()
	var tables int
	require.NoError(t, baselineDB.QueryRowContext(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname='public'").Scan(&tables))
	require.Zero(t, tables, "refuse a nonempty database; this test never cleans an existing application DB")
	require.NoError(t, applyMigrationsFS(ctx, baselineDB, os.DirFS(base)))
	before := september28MigrationReceipts(t, ctx, baselineDB)
	names := make([]string, 0, len(before))
	for name := range before {
		names = append(names, name)
	}
	sort.Strings(names)
	baselineDigest := sha256.New()
	for _, name := range names {
		fmt.Fprintf(baselineDigest, "%s%c%s\n", name, 0, before[name])
	}
	require.Equal(t, "aa5034f8164b58fec94947353be441991f7b0baeac48b9f7ba62f55013aed5c9", fmt.Sprintf("%x", baselineDigest.Sum(nil)), "baseline must exactly match the observed production migration receipts")
	require.Contains(t, before, "252_account_quality_unique_notx.sql")
	require.NotContains(t, before, "254_quality_bps_coexist.sql", "baseline must precede this release")
	require.NotContains(t, before, "253_pelican_group_tests.sql", "baseline must be exported from deployed root HEAD")
	_, err = baselineDB.ExecContext(ctx, `CREATE TABLE september28_old_columns AS
 SELECT table_name,column_name,data_type,udt_name,is_nullable,character_maximum_length,numeric_precision,numeric_scale
 FROM information_schema.columns WHERE table_schema='public'`)
	require.NoError(t, err)

	// Seed using the old application's SQL contract before expanding the schema.
	var account, quarantine int64
	require.NoError(t, baselineDB.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type,credentials)
 VALUES ('september28-test','openai','oauth','{}') RETURNING id`).Scan(&account))
	require.NoError(t, baselineDB.QueryRowContext(ctx, `INSERT INTO scheduled_test_plans(account_id,model_id,cron_expression,enabled,max_results,auto_recover,pelican_config)
 VALUES ($1,'migration-test','*/30 * * * *',false,100,false,'{"quality":{"action":"disable_scheduling"}}') RETURNING id`, account).Scan(&quarantine))
	_, err = baselineDB.ExecContext(ctx, `INSERT INTO scheduled_test_plans(account_id,pelican_config)
 VALUES ($1,'{"quality":{"action":"enable_bps"}}')`, account)
	september28RequireUniqueViolation(t, err)

	// DSN startup options apply to every pooled connection, including the runner's
	// dedicated advisory-lock connection and every per-file transaction.
	q := u.Query()
	q.Set("lock_timeout", "100ms")
	q.Set("statement_timeout", "2s")
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer db.Close()
	var lockBudget, statementBudget string
	require.NoError(t, db.QueryRowContext(ctx, "SHOW lock_timeout").Scan(&lockBudget))
	require.NoError(t, db.QueryRowContext(ctx, "SHOW statement_timeout").Scan(&statementBudget))
	require.Equal(t, "100ms", lockBudget)
	require.Equal(t, "2s", statementBudget)

	holder, err := baselineDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer holder.Rollback()
	_, err = holder.ExecContext(ctx, "LOCK TABLE scheduled_test_plans IN SHARE MODE")
	require.NoError(t, err)
	started := time.Now()
	err = ApplyMigrations(ctx, db)
	require.ErrorContains(t, err, "254_quality_bps_coexist.sql")
	require.ErrorContains(t, err, "lock timeout")
	var pgErr *pq.Error
	require.True(t, errors.As(err, &pgErr))
	require.Equal(t, pq.ErrorCode("55P03"), pgErr.Code)
	require.Less(t, time.Since(started), 5*time.Second, "online run must abort promptly rather than wait for the held table lock")

	// Failed DROP/CREATE is one transaction: the old valid unique index remains,
	// the new index and failed file's receipt do not. Earlier files stay committed.
	var oldUnique, newIndex bool
	require.NoError(t, baselineDB.QueryRowContext(ctx, `SELECT i.indisvalid AND i.indisunique
 FROM pg_index i WHERE i.indexrelid='scheduled_test_quality_account_unique'::regclass`).Scan(&oldUnique))
	require.True(t, oldUnique)
	require.NoError(t, baselineDB.QueryRowContext(ctx, `SELECT to_regclass('scheduled_test_quality_account_scope_unique') IS NOT NULL`).Scan(&newIndex))
	require.False(t, newIndex)
	partial := september28MigrationReceipts(t, ctx, baselineDB)
	require.NotContains(t, partial, "254_quality_bps_coexist.sql")
	for _, name := range []string{"253_pelican_group_tests.sql", "254_channel_monitor_v2_candy.sql", "254_openai_oauth_reauth.sql"} {
		require.Contains(t, partial, name, "earlier committed migration receipt must survive the later lock failure")
	}
	for name, checksum := range before {
		require.Equal(t, checksum, partial[name], "existing receipt changed after failed migration: %s", name)
	}
	require.NoError(t, holder.Rollback())
	// The retained index still enforces its old account-only rule after rollback.
	_, err = baselineDB.ExecContext(ctx, `INSERT INTO scheduled_test_plans(account_id,pelican_config)
 VALUES ($1,'{"quality":{"action":"enable_bps"}}')`, account)
	september28RequireUniqueViolation(t, err)

	require.NoError(t, ApplyMigrations(ctx, db))
	after := september28MigrationReceipts(t, ctx, db)
	for name, checksum := range partial {
		require.Equal(t, checksum, after[name], "retry changed previously committed receipt: %s", name)
	}
	for _, name := range []string{"254_quality_bps_coexist.sql", "255_account_token_guard_v2.sql", "256_openai_oauth_reauth_proxy_override.sql", "257_openai_oauth_reauth_proxy_source.sql"} {
		require.Contains(t, after, name)
	}
	require.NoError(t, ApplyMigrations(ctx, db), "fully applied migration set must be idempotent")
	require.Equal(t, after, september28MigrationReceipts(t, ctx, db))
	var missing int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM september28_old_columns o WHERE NOT EXISTS (
 SELECT 1 FROM information_schema.columns c WHERE c.table_schema='public'
 AND c.table_name=o.table_name AND c.column_name=o.column_name AND c.data_type=o.data_type
 AND c.udt_name=o.udt_name AND c.is_nullable=o.is_nullable
 AND c.character_maximum_length IS NOT DISTINCT FROM o.character_maximum_length
 AND c.numeric_precision IS NOT DISTINCT FROM o.numeric_precision
 AND c.numeric_scale IS NOT DISTINCT FROM o.numeric_scale)`).Scan(&missing))
	require.Zero(t, missing, "old column type/nullability contracts must remain intact")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT to_regclass('scheduled_test_quality_account_unique') IS NOT NULL`).Scan(&oldUnique))
	require.False(t, oldUnique)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT i.indisvalid AND i.indisunique
 FROM pg_index i WHERE i.indexrelid='scheduled_test_quality_account_scope_unique'::regclass`).Scan(&newIndex))
	require.True(t, newIndex)

	var bps int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO scheduled_test_plans(account_id,pelican_config)
 VALUES ($1,'{"quality":{"action":"enable_bps"}}') RETURNING id`, account).Scan(&bps))
	require.NotEqual(t, quarantine, bps, "two scopes for one account must coexist")
	for _, action := range []string{"enable_bps", "disable_scheduling", "remove_groups", "remove_models"} {
		_, err = db.ExecContext(ctx, `INSERT INTO scheduled_test_plans(account_id,enabled,pelican_config)
 VALUES ($1,false,jsonb_build_object('quality',jsonb_build_object('action',$2::text)))`, account, action)
		september28RequireUniqueViolation(t, err)
	}
	// Old quarantine UPDATE/SELECT projections work even with a BPS sibling;
	// this checks the SQL contract, not execution of a historical Go binary.
	var id, readAccount int64
	var model, cron, action string
	var enabled, autoRecover bool
	var maxResults int
	require.NoError(t, db.QueryRowContext(ctx, `UPDATE scheduled_test_plans
 SET model_id='old-writer-updated',enabled=true,pelican_config='{"quality":{"action":"remove_groups"}}',updated_at=NOW()
 WHERE id=$1 RETURNING id`, quarantine).Scan(&id))
	require.Equal(t, quarantine, id)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id,account_id,model_id,cron_expression,enabled,max_results,auto_recover,
 pelican_config->'quality'->>'action' FROM scheduled_test_plans WHERE id=$1`, quarantine).
		Scan(&id, &readAccount, &model, &cron, &enabled, &maxResults, &autoRecover, &action))
	require.Equal(t, account, readAccount)
	require.Equal(t, "old-writer-updated", model)
	require.Equal(t, "*/30 * * * *", cron)
	require.True(t, enabled)
	require.Equal(t, 100, maxResults)
	require.False(t, autoRecover)
	require.Equal(t, "remove_groups", action)
	_, err = db.ExecContext(ctx, `UPDATE scheduled_test_plans SET pelican_config='{"quality":{"action":"enable_bps"}}' WHERE id=$1`, quarantine)
	september28RequireUniqueViolation(t, err)
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM scheduled_test_plans WHERE account_id=$1`, account).Scan(&count))
	require.Equal(t, 2, count, "rejected scope changes must not lose or overwrite either rule")
}

func september28MigrationReceipts(t *testing.T, ctx context.Context, db *sql.DB) map[string]string {
	t.Helper()
	rows, err := db.QueryContext(ctx, "SELECT filename,checksum FROM schema_migrations ORDER BY filename")
	require.NoError(t, err)
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var name, checksum string
		require.NoError(t, rows.Scan(&name, &checksum))
		result[name] = checksum
	}
	require.NoError(t, rows.Err())
	return result
}

func september28RequireUniqueViolation(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var pgErr *pq.Error
	require.True(t, errors.As(err, &pgErr))
	require.Equal(t, pq.ErrorCode("23505"), pgErr.Code)
}
