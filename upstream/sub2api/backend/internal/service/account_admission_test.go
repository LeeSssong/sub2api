package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type admissionGroupRepo struct {
	GroupRepository
	getByIDByID map[int64]*Group
}

func (r *admissionGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	g := r.getByIDByID[id]
	if g == nil {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

type admissionCreateRepo struct {
	AccountRepository
	createAccount   *Account
	bindGroupsCalls []int64
	destinations    []int64
	temporary       *int64
}

func (r *admissionCreateRepo) CreateWithAdmission(_ context.Context, a *Account, groups []int64, test *int64) error {
	r.createAccount = a
	r.destinations = append([]int64(nil), groups...)
	r.temporary = test
	a.ID = 8
	return nil
}
func TestAdmissionCreateIsIsolatedBeforePersistence(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic} {
		t.Run(platform, func(t *testing.T) {
			r := &admissionCreateRepo{}
			g := int64(1)
			s := &adminServiceImpl{accountRepo: r, groupRepo: &admissionGroupRepo{getByIDByID: map[int64]*Group{1: {ID: 1, Platform: platform, Status: StatusActive}, 2: {ID: 2, Platform: platform, Status: StatusActive}}}}
			a, err := s.CreateAccount(context.Background(), &CreateAccountInput{Name: "new", Platform: platform, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "local-fixture"}, GroupIDs: []int64{2}, SkipMixedChannelCheck: true, Admission: &AccountAdmissionInput{Enabled: true, TestGroupID: &g}})
			require.NoError(t, err)
			require.False(t, a.Schedulable)
			require.False(t, r.createAccount.Schedulable)
			require.Equal(t, []int64{2}, r.destinations)
			require.Equal(t, int64(1), *r.temporary)
			require.Empty(t, r.bindGroupsCalls)
		})
	}
}
func TestAdmissionValidation(t *testing.T) {
	g := int64(1)
	for _, tt := range []struct {
		name    string
		groups  []int64
		temp    *int64
		json    bool
		wantErr bool
	}{
		{"ordinary requires temporary", []int64{2}, nil, false, true}, {"json ungrouped allowed", []int64{2}, nil, true, false}, {"formal required", nil, &g, false, true}, {"overlap rejected", []int64{1}, &g, false, true}, {"duplicate rejected", []int64{2, 2}, &g, false, true}, {"platform rejected", []int64{3}, &g, false, true}, {"inactive rejected", []int64{4}, &g, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &admissionCreateRepo{}
			s := &adminServiceImpl{accountRepo: r, groupRepo: &admissionGroupRepo{getByIDByID: map[int64]*Group{1: {ID: 1, Platform: PlatformOpenAI, Status: StatusActive}, 2: {ID: 2, Platform: PlatformOpenAI, Status: StatusActive}, 3: {ID: 3, Platform: PlatformAnthropic, Status: StatusActive}, 4: {ID: 4, Platform: PlatformOpenAI, Status: "disabled"}}}}
			_, err := s.CreateAccount(context.Background(), &CreateAccountInput{Name: "new", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, GroupIDs: tt.groups, SkipMixedChannelCheck: true, AdmissionAllowUngrouped: tt.json, Admission: &AccountAdmissionInput{Enabled: true, TestGroupID: tt.temp}})
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, r.createAccount)
			} else {
				require.NoError(t, err)
				require.False(t, r.createAccount.Schedulable)
			}
		})
	}
}
func TestAdmissionRoundRequiresBothSuccessful(t *testing.T) {
	success := &ScheduledTestResult{Status: "success"}
	for _, tt := range []struct {
		name string
		a, b *ScheduledTestResult
		want string
	}{
		{"both pass", success, success, "passed"}, {"missing result", success, nil, "inconclusive"}, {"wrong candy", &ScheduledTestResult{Status: "failed", ErrorMessage: "answer_mismatch: expected 21"}, success, "failed"}, {"invalid pelican", success, &ScheduledTestResult{Status: "failed", ErrorMessage: "Model did not return HTML or SVG"}, "failed"}, {"transport inconclusive", success, &ScheduledTestResult{Status: "failed", ErrorMessage: "context deadline exceeded"}, "inconclusive"},
	} {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, AccountAdmissionOutcome(tt.a, tt.b)) })
	}
}

func TestAdmissionAntigravityUsesNativeMixedScheduling(t *testing.T) {
	s := &adminServiceImpl{groupRepo: &admissionGroupRepo{getByIDByID: map[int64]*Group{1: {ID: 1, Platform: PlatformAntigravity, Status: StatusActive}, 2: {ID: 2, Platform: PlatformAnthropic, Status: StatusActive}, 3: {ID: 3, Platform: PlatformGemini, Status: StatusActive}}}}
	temp := int64(1)
	input := &CreateAccountInput{Platform: PlatformAntigravity, Extra: map[string]any{"mixed_scheduling": true}, GroupIDs: []int64{2, 3}, Admission: &AccountAdmissionInput{Enabled: true, TestGroupID: &temp}}
	require.NoError(t, s.validateAdmission(context.Background(), input))
	input.Extra["mixed_scheduling"] = false
	require.Error(t, s.validateAdmission(context.Background(), input))
}
func TestAdmissionGateAlsoProtectsShadowCredentialUse(t *testing.T) {
	a := &Account{Status: StatusActive, Schedulable: true, Extra: map[string]any{AccountAdmissionBlockedKey: true}}
	require.False(t, a.IsSchedulable())
	require.False(t, a.IsCredentialUsableForShadow())
}

type admissionResultRepo struct {
	AccountAdmissionRepository
	outcome  string
	results  []*ScheduledTestResult
	ctxError error
}

func (r *admissionResultRepo) FinishAdmission(ctx context.Context, _ *AccountAdmissionJob, outcome string, a, b *ScheduledTestResult) error {
	r.ctxError = ctx.Err()
	r.outcome = outcome
	r.results = []*ScheduledTestResult{a, b}
	return nil
}
func TestAdmissionRoundRunsBothNativeQuestionsWithOneDeadline(t *testing.T) {
	repo := &admissionResultRepo{}
	s := &AccountAdmissionService{repo: repo}
	var kinds []string
	var deadline time.Time
	s.run = func(ctx context.Context, id int64, model string, cfg *PelicanTestConfig) (*ScheduledTestResult, error) {
		require.Equal(t, int64(8), id)
		require.Empty(t, model, "native platform default must choose the model")
		d, ok := ctx.Deadline()
		require.True(t, ok)
		if deadline.IsZero() {
			deadline = d
		} else {
			require.Equal(t, deadline, d)
		}
		require.Nil(t, cfg.Quality, "admission never invokes an AI judge")
		kinds = append(kinds, cfg.QuestionKind)
		if cfg.QuestionKind == "candy" {
			require.Equal(t, CandyPrompt, cfg.Prompt)
			return &ScheduledTestResult{Status: "success", ResponseText: "21"}, nil
		}
		return nil, context.DeadlineExceeded
	}
	s.runRound(context.Background(), &AccountAdmissionJob{AccountID: 8})
	require.Equal(t, []string{"candy", "pelican"}, kinds)
	require.Equal(t, "inconclusive", repo.outcome)
	require.Nil(t, repo.ctxError)
	require.Len(t, repo.results, 2)
	require.WithinDuration(t, time.Now().Add(8*time.Minute), deadline, time.Second)
}
