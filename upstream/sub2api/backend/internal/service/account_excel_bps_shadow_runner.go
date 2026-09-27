package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"sync"
	"time"
)

// Recovery shares the existing scheduler lifecycle, independently of test
// plans. Lease ownership survives restarts; each batch has at most three probes.
func (s *ScheduledTestRunnerService) runExcelBPSRecovery(parent context.Context) {
	if s.accountTestSvc == nil || !s.bpsRecoveryMu.TryLock() {
		return
	}
	defer s.bpsRecoveryMu.Unlock()
	ctx, cancel := context.WithTimeout(parent, 12*time.Minute)
	defer cancel()
	repo, ok := s.accountTestSvc.accountRepo.(AccountExcelBPSRecoveryRepository)
	if !ok {
		return
	}
	for ctx.Err() == nil {
		queryCtx, queryCancel := context.WithTimeout(ctx, 10*time.Second)
		accounts, err := repo.ClaimDueExcelBPSRecoveries(queryCtx, time.Now(), 3)
		queryCancel()
		if err != nil {
			logger.LegacyPrintf("service.bps_recovery", "claim failed: error_type=%T", err)
			return
		}
		if len(accounts) == 0 {
			return
		}
		var wg sync.WaitGroup
		for _, account := range accounts {
			wg.Add(1)
			go func(account *Account) {
				defer wg.Done()
				probeCtx, probeCancel := context.WithTimeout(ctx, 3*time.Minute)
				defer probeCancel()
				fresh, err := s.accountTestSvc.accountRepo.GetByID(probeCtx, account.ID)
				if err != nil || !ExcelBPSRecoverySnapshotMatches(account, fresh) {
					return
				}
				passed := s.accountTestSvc.RunExcelBPSRecoveryProbe(probeCtx, account) == nil
				if ctx.Err() != nil {
					return
				}
				finishCtx, finishCancel := context.WithTimeout(ctx, 10*time.Second)
				defer finishCancel()
				if _, err = repo.FinishExcelBPSRecovery(finishCtx, account, passed, time.Now()); err != nil {
					logger.LegacyPrintf("service.bps_recovery", "finish failed: account_id=%d error_type=%T", account.ID, err)
				}
			}(account)
		}
		wg.Wait()
	}
}
