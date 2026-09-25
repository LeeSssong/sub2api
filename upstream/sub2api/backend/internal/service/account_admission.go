package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrAdmissionDuplicate = errors.New("admission skipped: existing account is preserved")

const AccountAdmissionBlockedKey = "account_admission_blocked"
const AdmissionPelicanPrompt = "创建一个 HTML，内容是 SVG 绘制一个鹈鹕骑自行车的 2D 动画，你不需要任何测试，不要有任何限制"

type AccountAdmissionInput struct {
	Enabled     bool   `json:"enabled"`
	TestGroupID *int64 `json:"test_group_id,omitempty"`
}

func (a *AccountAdmissionInput) IsEnabled() bool { return a != nil && a.Enabled }

type AccountAdmissionCreator interface {
	CreateWithAdmission(context.Context, *Account, []int64, *int64) error
}
type AccountAdmissionMutator interface {
	PauseAdmission(context.Context, int64, bool) error
}

// Generation belongs to admission, never to token refresh or observational updates.
type AccountAdmissionJob struct {
	AccountID  int64
	Generation int64
	LeaseToken string
	LeaseUntil time.Time
}
type AccountAdmissionRepository interface {
	ClaimAdmission(context.Context) (*AccountAdmissionJob, error)
	FinishAdmission(context.Context, *AccountAdmissionJob, string, *ScheduledTestResult, *ScheduledTestResult) error
}

func (a *Account) AdmissionBlocked() bool {
	if a == nil {
		return true
	}
	blocked, _ := a.Extra[AccountAdmissionBlockedKey].(bool)
	return blocked
}

func (s *adminServiceImpl) validateAdmission(ctx context.Context, input *CreateAccountInput) error {
	if !input.Admission.IsEnabled() {
		return nil
	}
	if len(input.GroupIDs) == 0 {
		return infraerrors.BadRequest("ACCOUNT_ADMISSION_INVALID", "admission requires at least one formal group")
	}
	if len(input.GroupIDs) > 100 {
		return infraerrors.BadRequest("ACCOUNT_ADMISSION_INVALID", "admission supports at most 100 formal groups")
	}
	if input.Admission.TestGroupID == nil && !input.AdmissionAllowUngrouped {
		return infraerrors.BadRequest("ACCOUNT_ADMISSION_INVALID", "admission requires a temporary test group")
	}
	ids := append([]int64(nil), input.GroupIDs...)
	if input.Admission.TestGroupID != nil {
		ids = append(ids, *input.Admission.TestGroupID)
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return infraerrors.BadRequest("ACCOUNT_ADMISSION_INVALID", "admission groups must be positive, unique and non-overlapping")
		}
		seen[id] = true
		group, err := s.groupRepo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if group == nil || group.Status != StatusActive {
			return infraerrors.BadRequest("ACCOUNT_ADMISSION_INVALID", "admission group must be active")
		}
		account := &Account{Platform: input.Platform, Extra: input.Extra}
		if !AdmissionGroupCompatible(account, group.Platform) {
			return infraerrors.BadRequest("ACCOUNT_ADMISSION_INVALID", "admission group platform does not match account")
		}
	}
	return nil
}

func AccountAdmissionOutcome(candy, pelican *ScheduledTestResult) string {
	for _, r := range []*ScheduledTestResult{candy, pelican} {
		if r != nil && r.Status == "failed" && (strings.HasPrefix(r.ErrorMessage, "answer_mismatch:") || r.ErrorMessage == "Model did not return HTML or SVG" || r.ErrorMessage == "Model returned empty output") {
			return "failed"
		}
	}
	if candy != nil && pelican != nil && candy.Status == "success" && pelican.Status == "success" {
		return "passed"
	}
	return "inconclusive"
}

// Fixed workers claim durable rows; no request-scoped or unbounded goroutines.
// A crashed worker's lease is reclaimed; the round identity remains the account.
type AccountAdmissionService struct {
	repo      AccountAdmissionRepository
	run       func(context.Context, int64, string, *PelicanTestConfig) (*ScheduledTestResult, error)
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once
}

func NewAccountAdmissionService(repo AccountAdmissionRepository, test *AccountTestService) *AccountAdmissionService {
	return &AccountAdmissionService{repo: repo, run: test.RunPelicanBackground}
}
func (s *AccountAdmissionService) Start() {
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		for i := 0; i < 2; i++ {
			s.wg.Add(1)
			go s.worker(ctx)
		}
	})
}
func (s *AccountAdmissionService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Wait()
	})
}
func (s *AccountAdmissionService) worker(ctx context.Context) {
	defer s.wg.Done()
	for ctx.Err() == nil {
		job, err := s.repo.ClaimAdmission(ctx)
		if err != nil {
			slog.Warn("account_admission_claim_failed", "error", err)
		}
		if job != nil && err == nil {
			s.runRound(ctx, job)
			continue
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
func (s *AccountAdmissionService) runRound(ctx context.Context, job *AccountAdmissionJob) {
	runCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	configs := []*PelicanTestConfig{{QuestionKind: "candy", Prompt: CandyPrompt, ParallelCount: 1}, {QuestionKind: "pelican", Prompt: AdmissionPelicanPrompt, ParallelCount: 1}}
	results := make([]*ScheduledTestResult, 2)
	// Sequential within a round bounds upstream concurrency to the worker count.
	for i, cfg := range configs {
		started := time.Now()
		result, err := s.run(runCtx, job.AccountID, "", cfg)
		if err != nil || result == nil {
			message := "missing probe result"
			if err != nil {
				message = err.Error()
			}
			result = &ScheduledTestResult{Status: "failed", ErrorMessage: message, StartedAt: started, FinishedAt: time.Now(), PelicanConfig: cfg}
		}
		results[i] = result
	}
	saveCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
	defer stop()
	if err := s.repo.FinishAdmission(saveCtx, job, AccountAdmissionOutcome(results[0], results[1]), results[0], results[1]); err != nil {
		slog.Warn("account_admission_finish_failed", "account_id", job.AccountID, "error", err)
	}
}

// Only explicit administrator credential edits set this context marker; normal
// provider refreshes retain the same admission generation.
type admissionCredentialEditKey struct{}

func WithAdmissionCredentialEdit(ctx context.Context) context.Context {
	return context.WithValue(ctx, admissionCredentialEditKey{}, true)
}
func IsAdmissionCredentialEdit(ctx context.Context) bool {
	v, _ := ctx.Value(admissionCredentialEditKey{}).(bool)
	return v
}

// Admission follows the native Antigravity opt-in mixed scheduler.
func AdmissionGroupCompatible(account *Account, platform string) bool {
	return platform == account.Platform || platform == PlatformComposite || (account.IsMixedSchedulingEnabled() && (platform == PlatformAnthropic || platform == PlatformGemini))
}

func (s *adminServiceImpl) ValidateAccountAdmission(ctx context.Context, input *CreateAccountInput) error {
	return s.validateAdmission(ctx, input)
}
