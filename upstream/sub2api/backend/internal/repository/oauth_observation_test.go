package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/oauthobs"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type observationBatchArgument struct{}

func (observationBatchArgument) Match(v driver.Value) bool {
	raw, ok := v.(string)
	if !ok {
		return false
	}
	var batch []map[string]any
	if json.Unmarshal([]byte(raw), &batch) != nil || len(batch) != 1 {
		return false
	}
	e := batch[0]
	p, ok := e["payload"].(map[string]any)
	return ok && e["account_id"] == float64(42) && e["event_key"] == "probe-42" && e["event_type"] == "probe_result" && p["verdict"] == "healthy"
}

func TestOAuthObservationBatchUsesAtomicTypedStatement(t *testing.T) {
	db, m, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	m.ExpectExec("SELECT oauth_observation_append").WithArgs(observationBatchArgument{}).WillReturnResult(sqlmock.NewResult(0, 1))
	r := newAccountRepositoryWithSQL(nil, db, nil)
	err = r.WriteOAuthObservations(context.Background(), []oauthobs.Event{{AccountID: 42, Key: "probe-42", OccurredAt: time.Now(), Type: "probe_result", Payload: oauthobs.Payload{Verdict: "healthy"}}})
	require.NoError(t, err)
	require.NoError(t, m.ExpectationsWereMet())
}

type orderedObservationBatchArgument struct{}

func (orderedObservationBatchArgument) Match(v driver.Value) bool {
	raw, ok := v.(string)
	if !ok {
		return false
	}
	var batch []map[string]any
	if json.Unmarshal([]byte(raw), &batch) != nil || len(batch) != 3 {
		return false
	}
	return batch[0]["account_id"] == float64(1) && batch[0]["event_key"] == "one" &&
		batch[1]["account_id"] == float64(2) && batch[1]["event_key"] == "two-first" &&
		batch[2]["account_id"] == float64(2) && batch[2]["event_key"] == "two-second"
}

func TestOAuthObservationBatchOrdersAccountsWithoutMutatingCaller(t *testing.T) {
	db, m, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	m.ExpectExec("SELECT oauth_observation_append").WithArgs(orderedObservationBatchArgument{}).WillReturnResult(sqlmock.NewResult(0, 3))
	events := []oauthobs.Event{
		{AccountID: 2, Key: "two-first", Type: "selected"},
		{AccountID: 1, Key: "one", Type: "selected"},
		{AccountID: 2, Key: "two-second", Type: "request_outcome"},
	}
	r := newAccountRepositoryWithSQL(nil, db, nil)
	require.NoError(t, r.WriteOAuthObservations(context.Background(), events))
	require.Equal(t, "two-first", events[0].Key, "caller order must remain intact")
	require.NoError(t, m.ExpectationsWereMet())
}

func TestOAuthObservationHealthUpsertsLatestAndMinutePeak(t *testing.T) {
	db, m, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	m.ExpectExec("WITH latest AS").
		WithArgs("instance-a", sqlmock.AnyArg(), int64(10), int64(8), int64(2), int64(1), 9).
		WillReturnResult(sqlmock.NewResult(0, 1))
	r := newAccountRepositoryWithSQL(nil, db, nil)
	require.NoError(t, r.WriteOAuthObservationHealth(context.Background(), oauthobs.Health{
		InstanceID: "instance-a", ObservedAt: time.Now(), Enqueued: 10, Persisted: 8, Dropped: 2, WriteErrors: 1, QueueDepth: 9,
	}))
	require.NoError(t, m.ExpectationsWereMet())
}

// A failure to attribute a quality mutation must roll back before changing the
// account, rather than committing an unattributable automatic disable.
func TestOAuthObservationQualityAttributionAtomic(t *testing.T) {
	db, m, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	p := &service.ScheduledTestPlan{ID: 7, AccountID: 8, UpdatedAt: time.Now(), PelicanConfig: &service.PelicanTestConfig{Quality: &service.QualityPolicy{Action: "disable_scheduling"}}}
	until := time.Now().Add(time.Minute)
	m.ExpectBegin()
	m.ExpectQuery("SELECT enabled AND updated_at").WithArgs(p.ID, p.UpdatedAt, until).WillReturnRows(sqlmock.NewRows([]string{"valid"}).AddRow(true))
	m.ExpectExec("SELECT set_config").WithArgs("7", "failed").WillReturnError(errors.New("database unavailable"))
	m.ExpectRollback()
	_, err = NewScheduledTestPlanRepository(db).ApplyQualityOutcome(context.Background(), p, until, "failed")
	require.Error(t, err)
	require.NoError(t, m.ExpectationsWereMet())
}
