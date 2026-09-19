package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAccountSchedulingPriorityForGroupUsesMatchingGroupRow(t *testing.T) {
	account := projectionTestAccount(301)
	account.Priority = 8
	account.AccountGroups = []AccountGroup{
		{AccountID: account.ID, GroupID: 77, Priority: 2},
		{AccountID: account.ID, GroupID: 88, Priority: 6},
	}

	group77 := int64(77)
	group88 := int64(88)
	missingGroup := int64(99)
	require.Equal(t, 2, openAIAccountSchedulingPriorityForGroup(account, &group77))
	require.Equal(t, 6, openAIAccountSchedulingPriorityForGroup(account, &group88))
	require.Equal(t, 8, openAIAccountSchedulingPriorityForGroup(account, &missingGroup))
	require.Equal(t, 8, openAIAccountSchedulingPriorityForGroup(account, nil))
}

func candidateIDs(candidates []openAIAccountCandidateScore) []int64 {
	ids := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.account != nil {
			ids = append(ids, candidate.account.ID)
		}
	}
	return ids
}

func projectionCandidatePriorityForTest(scheduler *defaultOpenAIAccountScheduler, groupID int64, account *Account, now time.Time) int {
	plan := scheduler.buildOpenAIAccountLoadPlanAtWithPolicy(context.Background(), OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI}, []*Account{account}, map[int64]*AccountLoadInfo{account.ID: {AccountID: account.ID}}, now, nil, false)
	return plan.preTopKCandidates[0].priority
}

func TestOpenAIAccountSchedulerProjectionAppliesGrokFreeQuotaSoftGateAndMarksModelParityUnknown(t *testing.T) {
	scheduler, _, _, _ := newOpenAIAccountSchedulerProjectionTestScheduler(t, "")
	scheduler.service.cfg = projectionGrokFreeQuotaTestConfig()
	scheduler.service.usageLogRepo = &struct{ UsageLogRepository }{}
	now := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	free := projectionTestAccount(103)
	free.Platform = PlatformGrok
	free.Type = AccountTypeOAuth
	free.Credentials = map[string]any{"subscription_tier": "free"}
	scheduler.grokFreeQuotaGateCache.Store(free.ID, grokFreeQuotaGateCacheEntry{tokens: 480_000, checkedAt: time.Now().UTC(), known: true})

	projection, err := scheduler.Project(context.Background(), OpenAIAccountSchedulerProjectionRequest{
		GroupID: 77, Platform: PlatformGrok, SnapshotAt: now, Accounts: []*Account{free},
		LoadMap: map[int64]*AccountLoadInfo{free.ID: {AccountID: free.ID}},
	})

	require.NoError(t, err)
	require.Equal(t, "unknown", projection.ModelQuotaParity)
	candidate := projectionCandidateByID(projection.Candidates, free.ID)
	require.False(t, candidate.Eligible)
	require.Equal(t, AccountMonitorReasonNotEligible, candidate.PrimaryReasonCode)
}

type projectionUsageLogRepoStub struct {
	UsageLogRepository
}

func (*projectionUsageLogRepoStub) GetAccountWindowStats(context.Context, int64, time.Time) (*usagestats.AccountStats, error) {
	return &usagestats.AccountStats{}, nil
}

func TestOpenAIGatewayServiceProjectionReusesLiveSchedulerGrokFreeQuotaCache(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
	now := time.Now().UTC()
	free := projectionTestAccount(104)
	free.Platform = PlatformGrok
	free.Type = AccountTypeOAuth
	free.Credentials = map[string]any{"subscription_tier": "free"}
	service := &OpenAIGatewayService{
		cfg:              projectionGrokFreeQuotaTestConfig(),
		usageLogRepo:     &projectionUsageLogRepoStub{},
		rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"),
	}
	liveScheduler, ok := service.getOpenAIAccountScheduler(context.Background()).(*defaultOpenAIAccountScheduler)
	require.True(t, ok)
	liveScheduler.grokFreeQuotaGateCache.Store(free.ID, grokFreeQuotaGateCacheEntry{
		tokens: 480_000, checkedAt: now, known: true,
	})

	projection, err := service.Project(context.Background(), OpenAIAccountSchedulerProjectionRequest{
		GroupID: 77, Platform: PlatformGrok, SnapshotAt: now, Accounts: []*Account{free},
		LoadMap: map[int64]*AccountLoadInfo{free.ID: {AccountID: free.ID}},
	})

	require.NoError(t, err)
	candidate := projectionCandidateByID(projection.Candidates, free.ID)
	require.False(t, candidate.Eligible)
	require.Equal(t, AccountMonitorReasonNotEligible, candidate.PrimaryReasonCode)
}

func TestOpenAIAccountSchedulerProjection_UsesOperatorStrategyLabels(t *testing.T) {
	tests := []struct {
		name   string
		preset OpenAISchedulerPreset
		label  string
	}{
		{name: "special offer", preset: OpenAISchedulerPresetSpecialOffer, label: "体验优先"},
		{name: "balanced", preset: OpenAISchedulerPresetBalanced, label: "体验均衡"},
		{name: "pro", preset: OpenAISchedulerPresetPro, label: "利润优先"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.label, schedulerProjectionPolicyLabel(OpenAISchedulerGroupPolicy{Preset: tt.preset}, true))
		})
	}
}

func TestOpenAIAccountSchedulerProjectionReadOnlyEligibilityUsesLazyRuntimeGetters(t *testing.T) {
	service := &OpenAIGatewayService{}
	now := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	account := projectionTestAccount(902)
	proxyID := int64(903)
	account.ProxyID = &proxyID
	model := "gpt-5.4-mini"

	require.False(t, service.isOpenAIAccountModelRuntimeBlockedAtReadOnly(account, model, now))
	require.NotNil(t, service.openaiModelTransient)
	service.openaiModelTransient.entries[openAIAccountModelKey{AccountID: account.ID, Model: model}] = openAIAccountModelTransientEntry{blockUntil: now.Add(time.Hour)}
	require.True(t, service.isOpenAIAccountModelRuntimeBlockedAtReadOnly(account, model, now))

	require.False(t, service.isOpenAIProxyStreamQuarantinedAtReadOnly(context.Background(), account, now))
	require.NotNil(t, service.openaiProxyStreamCircuit)
	service.openaiProxyStreamCircuit.entries[proxyID] = openAIProxyStreamCircuitEntry{blockedUntil: now.Add(time.Hour)}
	require.True(t, service.isOpenAIProxyStreamQuarantinedAtReadOnly(context.Background(), account, now))
}

func TestOpenAIAccountSchedulerProjectionGrokQuotaEligibilityDoesNotRefreshCache(t *testing.T) {
	scheduler, _, _, _ := newOpenAIAccountSchedulerProjectionTestScheduler(t, "")
	scheduler.service.cfg = projectionGrokFreeQuotaTestConfig()
	repo := &projectionRefreshObservingUsageLogRepo{refreshStarted: make(chan struct{})}
	scheduler.service.usageLogRepo = repo
	now := time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC)
	free := projectionTestAccount(203)
	free.Platform = PlatformGrok
	free.Type = AccountTypeOAuth
	free.Credentials = map[string]any{"subscription_tier": "free"}

	projection, err := scheduler.Project(context.Background(), OpenAIAccountSchedulerProjectionRequest{
		GroupID: 77, Platform: PlatformGrok, SnapshotAt: now, Accounts: []*Account{free},
		LoadMap: map[int64]*AccountLoadInfo{free.ID: {AccountID: free.ID}},
	})

	require.NoError(t, err)
	require.Equal(t, "unknown", projection.ModelQuotaParity)
	require.True(t, projectionCandidateByID(projection.Candidates, free.ID).Eligible)
	require.Never(t, func() bool {
		select {
		case <-repo.refreshStarted:
			return true
		default:
			return false
		}
	}, 150*time.Millisecond, 10*time.Millisecond)
}

func TestOpenAIAdvancedSchedulerRuntimeSettingsPreservesContextCancellation(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)

	repo := &projectionContextAwareSettingRepo{openAIAdvancedSchedulerSettingRepoStub: &openAIAdvancedSchedulerSettingRepoStub{}}
	cfg := &config.Config{}
	service := &OpenAIGatewayService{
		cfg:              cfg,
		rateLimitService: &RateLimitService{settingService: NewSettingService(repo, cfg)},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = service.openAIAdvancedSchedulerRuntimeSettings(ctx)
	require.ErrorIs(t, repo.observedErr, context.Canceled)
	require.Nil(t, openAIAdvancedSchedulerSettingCache.Load())
}

func TestOpenAIAccountSchedulerProjectionStrategyDiffIgnoresEligibilityDifferences(t *testing.T) {
	first := &Account{ID: 501}
	second := &Account{ID: 502}
	ranked := []openAIAccountCandidateScore{{account: first}}
	// The quality order contains an account that the scheduler correctly
	// excluded. Only compare the common eligible intersection; otherwise the
	// remaining account would be incorrectly labelled as a policy mismatch.
	require.False(t, schedulerProjectionOrderDiffersFromQualityOrder(ranked, []int64{second.ID, first.ID}))
}

func TestOpenAIAccountSchedulerProjection_LabelsEqualCustomBusinessPrioritiesAsBalanced(t *testing.T) {
	require.Equal(t, "体验均衡", schedulerProjectionPolicyLabel(OpenAISchedulerGroupPolicy{
		Priority: OpenAISchedulerBusinessPriority{Profit: 1, TTFT: 1, Latency: 1},
	}, true))
}

func TestOpenAIAccountSchedulerProjection_LabelsLatencyBusinessPriorityAsGenerationExperience(t *testing.T) {
	require.Equal(t, "生成体验优先", schedulerProjectionPolicyLabel(OpenAISchedulerGroupPolicy{
		Priority: OpenAISchedulerBusinessPriority{Profit: 3, TTFT: 2, Latency: 1},
	}, true))
	require.Equal(t, "利润优先", schedulerProjectionPolicyLabel(OpenAISchedulerGroupPolicy{
		Priority: OpenAISchedulerBusinessPriority{Profit: 1, TTFT: 3, Latency: 3},
	}, true))
}

type projectionFailingSettingRepo struct {
	*openAIAdvancedSchedulerSettingRepoStub
	err error
}

type projectionRefreshObservingUsageLogRepo struct {
	UsageLogRepository
	refreshStarted chan struct{}
	once           sync.Once
}

func (r *projectionRefreshObservingUsageLogRepo) GetAccountWindowStats(context.Context, int64, time.Time) (*usagestats.AccountStats, error) {
	r.once.Do(func() { close(r.refreshStarted) })
	return &usagestats.AccountStats{}, nil
}

type projectionContextAwareSettingRepo struct {
	*openAIAdvancedSchedulerSettingRepoStub
	observedErr error
}

func (r *projectionContextAwareSettingRepo) GetMultiple(ctx context.Context, _ []string) (map[string]string, error) {
	r.observedErr = ctx.Err()
	if r.observedErr != nil {
		return nil, r.observedErr
	}
	return map[string]string{}, nil
}

func (r *projectionFailingSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, r.err
}

func newOpenAIAccountSchedulerProjectionTestScheduler(t *testing.T, priorityJSON string) (*defaultOpenAIAccountScheduler, *schedulerTestGatewayCache, *[]int64, *[]int64) {
	t.Helper()
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)

	values := map[string]string{openAIAdvancedSchedulerSettingKey: "true"}
	if priorityJSON != "" {
		values[SettingKeyOpenAIAdvancedSchedulerGroupOverrides] = `{"77":{"priority":` + priorityJSON + `,"operations":{"balance":"standard","peak_protection":"strict","session_continuity":"standard"}}}`
	}
	repo := &openAIAdvancedSchedulerSettingRepoStub{values: values}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate = 1
	cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT = 1
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{}}
	acquiredIDs := []int64{}
	releasedIDs := []int64{}
	service := &OpenAIGatewayService{
		cache:              cache,
		cfg:                cfg,
		rateLimitService:   &RateLimitService{settingService: NewSettingService(repo, cfg)},
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquiredIDs: &acquiredIDs, releasedIDs: &releasedIDs}),
	}
	return &defaultOpenAIAccountScheduler{service: service, stats: newOpenAIAccountRuntimeStats()}, cache, &acquiredIDs, &releasedIDs
}

func projectionTestAccount(id int64) *Account {
	return &Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}
}

func projectionTestUpstreamBillingExtra(now time.Time, rate float64) map[string]any {
	return map[string]any{
		UpstreamBillingProbeExtraKey: map[string]any{
			"status":        UpstreamBillingProbeStatusOK,
			"received_at":   now.Add(-10 * time.Minute).Format(time.RFC3339Nano),
			"fresh_until":   now.Add(time.Hour).Format(time.RFC3339Nano),
			"next_probe_at": now.Add(20 * time.Minute).Format(time.RFC3339Nano),
			"data": map[string]any{
				"billing_scope":             "token",
				"peak_rate_enabled":         false,
				"resolved_rate_multiplier":  rate,
				"effective_rate_multiplier": rate,
			},
		},
	}
}

func projectionGrokFreeQuotaTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Gateway.Grok.FreeQuotaSoftGateEnabled = true
	cfg.Gateway.Grok.FreeQuotaTokenLimit = 500_000
	cfg.Gateway.Grok.FreeQuotaSoftGatePercent = 95
	cfg.Gateway.Grok.FreeQuotaWindowHours = 24
	cfg.Gateway.Grok.FreeQuotaStatsCacheSeconds = 60
	return cfg
}

func projectionAccountIDs(candidates []OpenAIAccountSchedulerProjectionCandidate) []int64 {
	ids := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.AccountID)
	}
	return ids
}

func projectionCandidateByID(candidates []OpenAIAccountSchedulerProjectionCandidate, accountID int64) OpenAIAccountSchedulerProjectionCandidate {
	for _, candidate := range candidates {
		if candidate.AccountID == accountID {
			return candidate
		}
	}
	return OpenAIAccountSchedulerProjectionCandidate{}
}
