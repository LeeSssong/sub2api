package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestTestMainOnlineMigrationSnapshotCompatibility(t *testing.T) {
	dsn, baseline := os.Getenv("TEST_MAIN_ONLINE_PG_DSN"), os.Getenv("TEST_MAIN_ONLINE_BASE")
	if dsn == "" || baseline == "" {
		t.Skip("requires disposable local database and production baseline migrations")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var tables int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname='public'").Scan(&tables))
	require.Zero(t, tables, "refuse nonempty database")
	require.NoError(t, applyMigrationsFS(ctx, db, os.DirFS(baseline)))
	for table, filename := range map[string]string{"account_monitor_v4_snapshots": "238_monitor_v4_p50.sql", "account_monitor_results": "240_manual_probe_group_scope.sql"} {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE")
		require.NoError(t, err)
		content, err := migrations.FS.ReadFile(filename)
		require.NoError(t, err)
		started := time.Now()
		err = applyMigrationsFS(ctx, db, fstest.MapFS{filename: &fstest.MapFile{Data: content}})
		require.ErrorContains(t, err, "lock timeout")
		require.Less(t, time.Since(started), 2*time.Second, "DDL must not queue behind business queries")
		require.NoError(t, tx.Rollback())
	}
	require.NoError(t, ApplyMigrations(ctx, db))
	var operational bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='account_monitor_v4_snapshots' AND column_name='current_operational')`).Scan(&operational))
	require.True(t, operational, "old production binaries must keep their column")
	window := monitorV4StoredWindowFixture()
	repo := NewAccountMonitorRepository(db).(*accountMonitorRepository)
	require.NoError(t, repo.ReplaceMonitorV4Snapshots(ctx, window.SnapshotID, []service.MonitorV4StoredWindow{window}))
	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&count))
	require.Equal(t, 366, count)
	loaded, err := repo.LoadLatestMonitorV4Snapshot(ctx, window.Window)
	require.NoError(t, err)
	require.Equal(t, window.Groups[7].RealRequestCount, loaded.Groups[7].RealRequestCount)
	// The previous reader can still select its complete old contract after new writes.
	require.NoError(t, db.QueryRowContext(ctx, `SELECT current_operational FROM account_monitor_v4_snapshots WHERE group_id=7`).Scan(&operational))
	require.False(t, operational)
	_, err = db.ExecContext(ctx, `UPDATE account_monitor_v4_snapshots SET current_operational=TRUE, ttft_p50_ms=NULL, latency_p50_ms=NULL`)
	require.NoError(t, err)
	_, err = repo.LoadLatestMonitorV4Snapshot(ctx, window.Window)
	require.NoError(t, err, "new reader accepts old worker rows")
	require.NoError(t, ApplyMigrations(ctx, db), "migration retry is idempotent")
	_, err = db.ExecContext(ctx, `ALTER TABLE account_monitor_v4_snapshots DROP COLUMN current_operational`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE filename='265_monitor_v4_legacy_default.sql'`)
	require.NoError(t, err)
	compatibilityMigration, err := migrations.FS.ReadFile("265_monitor_v4_legacy_default.sql")
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFS(ctx, db, fstest.MapFS{
		"265_monitor_v4_legacy_default.sql": &fstest.MapFile{Data: compatibilityMigration},
	}), "conditional default accepts the existing test station schema")
	require.NoError(t, repo.ReplaceMonitorV4Snapshots(ctx, window.SnapshotID, []service.MonitorV4StoredWindow{window}))
	_, err = repo.LoadLatestMonitorV4Snapshot(ctx, window.Window)
	require.NoError(t, err)
}
