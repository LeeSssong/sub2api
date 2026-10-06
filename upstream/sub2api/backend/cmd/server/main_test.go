package main

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRunMigrationsOnlyDoesNotStartApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("migration-marker").WillReturnResult(sqlmock.NewResult(0, 1))

	called := false
	err = runMigrationsOnly(context.Background(), db, func(ctx context.Context, actual *sql.DB) error {
		called = true
		_, err := actual.ExecContext(ctx, "migration-marker")
		return err
	})
	require.NoError(t, err)
	require.True(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOnlineMigrationDSNHasShortLockTimeout(t *testing.T) {
	cfg := &config.Config{}
	cfg.Database.Host = "postgres"
	cfg.Database.Port = 5432
	cfg.Database.User = "sub2api"
	cfg.Database.DBName = "sub2api"
	cfg.Database.SSLMode = "disable"
	require.Contains(t, onlineMigrationDSN(cfg), "lock_timeout=100ms")
	require.Contains(t, onlineMigrationDSN(cfg), "statement_timeout=2s")
}

func TestRunStartup_APIFailsClosedBeforeSetupWhenSetupIsNeeded(t *testing.T) {
	setupCalls := 0
	actions := startupActions{
		needsSetup: func() bool {
			return true
		},
		autoSetupEnabled: func() bool {
			setupCalls++
			return true
		},
		autoSetup: func() error {
			setupCalls++
			return nil
		},
		runSetupServer: func() {
			setupCalls++
		},
		runMainServer: func() {},
	}

	err := runStartup(config.ProcessRoleAPI, false, actions)

	require.ErrorContains(t, err, "api process role cannot run setup")
	require.Zero(t, setupCalls, "API startup must reject setup before invoking any setup callback")
}

func TestRunStartup_APIFailsClosedBeforeCLISetup(t *testing.T) {
	setupCalls := 0
	actions := startupActions{
		runCLI: func() error {
			setupCalls++
			return nil
		},
	}

	err := runStartup(config.ProcessRoleAPI, true, actions)

	require.ErrorContains(t, err, "api process role cannot run setup")
	require.Zero(t, setupCalls, "API startup must reject CLI setup before invoking it")
}
