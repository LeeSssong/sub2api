package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type bpsRecoveryRunnerRepo struct {
	AccountRepository
	AccountExcelBPSRecoveryRepository
	account  *Account
	claimed  bool
	finished int
	passed   bool
	reads    int
	changeAt int
}

func (r *bpsRecoveryRunnerRepo) GetByID(context.Context, int64) (*Account, error) {
	r.reads++
	if r.changeAt > 0 && r.reads >= r.changeAt {
		a := *r.account
		a.Status = "disabled"
		return &a, nil
	}
	return r.account, nil
}
func (r *bpsRecoveryRunnerRepo) ClaimDueExcelBPSRecoveries(context.Context, time.Time, int) ([]*Account, error) {
	if r.claimed {
		return nil, nil
	}
	r.claimed = true
	return []*Account{r.account}, nil
}
func (r *bpsRecoveryRunnerRepo) FinishExcelBPSRecovery(_ context.Context, _ *Account, passed bool, _ time.Time) (bool, error) {
	r.finished++
	r.passed = passed
	return true, nil
}
func TestExcelBPSRecoveryRunnerRechecksBetweenProbeSteps(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		changeAt, wantRequests int
		pass                   bool
	}{{"pass", 0, 3, true}, {"changed before probe", 1, 0, false}, {"changed after text", 3, 1, false}, {"changed after tool", 4, 2, false}} {
		t.Run(tc.name, func(t *testing.T) {
			a := excelAccount()
			a.Extra[ExcelBPSShadowRecoveryKey] = true
			a.Extra[ExcelBPSRecoveryKey] = map[string]any{"active": true, "lease_token": "owner", "generation": "generation"}
			repo := &bpsRecoveryRunnerRepo{account: a, changeAt: tc.changeAt}
			upstream := &bpsProbeUpstream{}
			testSvc := bpsProbeTestService(upstream)
			testSvc.accountRepo = repo
			runner := &ScheduledTestRunnerService{accountTestSvc: testSvc}
			runner.runExcelBPSRecovery(context.Background())
			require.Len(t, upstream.bodies, tc.wantRequests)
			require.Equal(t, tc.pass, repo.passed)
			if tc.changeAt == 1 {
				require.Zero(t, repo.finished)
			} else {
				require.Equal(t, 1, repo.finished)
			}
			require.True(t, a.IsExcelBPSDegraded())
		})
	}
}
