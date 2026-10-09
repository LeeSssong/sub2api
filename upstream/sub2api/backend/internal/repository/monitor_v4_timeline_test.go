package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMonitorV4TimelineBuckets(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := &accountMonitorRepository{db: db}
	end := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	start := end.Add(-time.Hour)
	mock.ExpectQuery("WITH scopes AS").WithArgs(start, end, "5m0s", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"group_id", "bucket_start", "requests", "successes", "cache_hit_rate", "ttft_p50_ms"}).AddRow(7, start, 10, 9, 0.75, 1250).AddRow(7, start.Add(5*time.Minute), 0, 0, nil, nil))
	mock.ExpectQuery("WITH scoped_plans AS").WithArgs(sqlmock.AnyArg(), start, end).WillReturnRows(sqlmock.NewRows([]string{"group_id", "plan_id", "round_id", "finished_at", "status", "error_message", "config", "judgment", "answered"}))
	rows, err := r.ReadMonitorV4Timeline(context.Background(), []int64{7}, start, end, 5*time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, 0.75, *rows[0].CacheHitRate)
	require.Equal(t, 1250.0, *rows[0].TTFTP50MS)
	require.Nil(t, rows[1].CacheHitRate)
	require.Nil(t, rows[1].TTFTP50MS)
	require.Equal(t, 9, rows[0].SuccessCount)
	require.NoError(t, mock.ExpectationsWereMet())
	_, err = r.ReadMonitorV4Timeline(context.Background(), []int64{7}, start, end, time.Second)
	require.Error(t, err)
}
