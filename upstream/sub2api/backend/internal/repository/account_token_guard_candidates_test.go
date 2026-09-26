package repository

import (
	"context"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestTokenGuardCandidateQueryIncludesAllStatuses(t *testing.T) {
	for _, groups := range [][]int64{nil, {7, 9}} {
		t.Run(fmt.Sprint(groups), func(t *testing.T) {
			matcher := sqlmock.QueryMatcherFunc(func(_ string, query string) error {
				where := strings.SplitN(query, " WHERE ", 2)
				if len(where) != 2 {
					return fmt.Errorf("missing scope predicates: %s", query)
				}
				if strings.Contains(where[1], `"status"`) || strings.Contains(where[1], `"schedulable"`) {
					return fmt.Errorf("guard hides error/disabled accounts: %s", query)
				}
				for _, field := range []string{`"deleted_at" IS NULL`, `"platform" =`, `"type" =`, `"parent_account_id" IS NULL`} {
					if !strings.Contains(where[1], field) {
						return fmt.Errorf("missing %s", field)
					}
				}
				if len(groups) > 0 && !strings.Contains(where[1], `"group_id" IN`) {
					return fmt.Errorf("missing group scope: %s", query)
				}
				return nil
			})
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
			require.NoError(t, err)
			defer db.Close()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			repo := &accountRepository{client: client}
			mock.ExpectQuery("guard full status query").WillReturnRows(sqlmock.NewRows([]string{"id"}))
			accounts, err := repo.ListTokenGuardCandidates(context.Background(), groups)
			require.NoError(t, err)
			require.Empty(t, accounts)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
