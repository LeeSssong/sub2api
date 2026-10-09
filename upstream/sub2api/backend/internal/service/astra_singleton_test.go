package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type astraSingletonSettingsRepo struct {
	SettingRepository
	raw     string
	readErr error
}

func (r *astraSingletonSettingsRepo) GetValue(context.Context, string) (string, error) {
	return r.raw, r.readErr
}
func (r *astraSingletonSettingsRepo) Set(_ context.Context, _ string, raw string) error {
	r.raw = raw
	return nil
}

type astraSingletonRuntime struct {
	HTTPUpstream
	mu       sync.Mutex
	cfg      *config.Config
	prepared int
	verified int
	snapshot AstraGatewayRuntime
}

func (r *astraSingletonRuntime) SetAstraGatewayPreparer(func(context.Context, int64) error) {}
func (r *astraSingletonRuntime) PrepareAstraGateway(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prepared++
	r.snapshot = AstraGatewayRuntime{Revision: r.cfg.AstraRouting(ctx).Revision}
	return nil
}
func (r *astraSingletonRuntime) VerifyAstraGatewayTarget(_ context.Context, _ *http.Request, _ string, id int64, _ int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verified++
	expiry := time.Now().Add(5 * time.Minute)
	r.snapshot.Targets = append(r.snapshot.Targets, AstraRouteStatus{AccountID: id, State: "ready", Reason: "target_probe_passed", ExpiresAt: &expiry})
	return nil
}
func (r *astraSingletonRuntime) AstraGatewaySnapshot(context.Context) AstraGatewayRuntime {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshot
}

func TestAstraSingletonRolesAndExternalSettingsRefresh(t *testing.T) {
	settingsRepo := &astraSingletonSettingsRepo{}
	apiCfg := &config.Config{Server: config.ServerConfig{ProcessRole: config.ProcessRoleAPI}}
	workerCfg := &config.Config{Server: config.ServerConfig{ProcessRole: config.ProcessRoleWorker}}
	apiSettings := NewSettingService(settingsRepo, apiCfg)
	workerSettings := NewSettingService(settingsRepo, workerCfg)
	require.False(t, workerCfg.AstraRouting(t.Context()).CookiePool.Enabled, "prime stale worker cache before API save")
	target := &Account{ID: 300, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Credentials: map[string]any{"access_token": "fixture"}}
	accounts := &astraSchedulingRepo{accounts: map[int64]*Account{300: target}}
	runtime := &astraSingletonRuntime{cfg: workerCfg}
	worker := &AccountTestService{cfg: workerCfg, settingService: workerSettings, accountRepo: accounts, httpUpstream: runtime, openaiGatewayService: &OpenAIGatewayService{}}
	api := &AccountTestService{cfg: apiCfg, settingService: apiSettings, accountRepo: accounts, httpUpstream: runtime}
	saved, err := apiSettings.SetAstraRouting(t.Context(), config.AstraRoutingSettings{AccountScheduling: true, CookiePool: config.CodexGatewayPinConfig{Enabled: true, SourceAccountIDs: []int64{299}, TargetAccountIDs: []int64{300}}})
	require.NoError(t, err)
	api.StartAstraAutomaticSetup(saved)
	api.runAstraSingletonCycle(t.Context())
	api.syncAstraAccountScheduling(t.Context())
	stop := api.startAstraAccountScheduling()
	stop()
	require.Empty(t, accounts.writes, "API cannot write scheduling, even when called directly")
	runtime.mu.Lock()
	require.Zero(t, runtime.prepared)
	runtime.mu.Unlock()
	worker.runAstraSingletonCycle(t.Context())
	require.Eventually(t, func() bool {
		worker.astraSetupMu.Lock()
		defer worker.astraSetupMu.Unlock()
		return worker.astraSetupStatus.State == "ready"
	}, time.Second, time.Millisecond)
	worker.runAstraSingletonCycle(t.Context())
	require.Equal(t, saved, workerCfg.AstraRouting(t.Context()), "worker bypasses stale request cache to discover API save")
	require.True(t, target.Schedulable, "worker schedules from its own verification result")
	runtime.mu.Lock()
	require.Equal(t, 1, runtime.prepared)
	require.Equal(t, 1, runtime.verified)
	runtime.mu.Unlock()
	// A second external revision cancels/replaces preparation on this same worker.
	saved.CookiePool.IPAffinity = true
	updated, err := apiSettings.SetAstraRouting(t.Context(), saved)
	require.NoError(t, err)
	worker.runAstraSingletonCycle(t.Context())
	require.Eventually(t, func() bool {
		worker.astraSetupMu.Lock()
		defer worker.astraSetupMu.Unlock()
		return worker.astraSetupStatus.Revision == updated.Revision && worker.astraSetupStatus.State == "ready"
	}, time.Second, time.Millisecond)
	runtime.mu.Lock()
	require.Equal(t, 2, runtime.prepared)
	require.Equal(t, 2, runtime.verified)
	runtime.mu.Unlock()
	before := len(accounts.writes)
	settingsRepo.readErr = errors.New("offline")
	worker.runAstraSingletonCycle(t.Context())
	require.Len(t, accounts.writes, before, "DB read failure cannot act on stale configuration")
	worker.StopAstraAutomaticSetup()
}

func TestAstraSingletonRefreshPreservesSchedulingOnlyRevision(t *testing.T) {
	repo := &astraSingletonSettingsRepo{}
	cfg := &config.Config{}
	settings := NewSettingService(repo, cfg)
	value := config.AstraRoutingSettings{Revision: "same", AccountScheduling: false}
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	repo.raw = string(raw)
	require.False(t, cfg.AstraRouting(t.Context()).AccountScheduling)
	value.AccountScheduling = true
	value.SchedulingMode = "model"
	raw, err = json.Marshal(value)
	require.NoError(t, err)
	repo.raw = string(raw)
	refreshed, err := settings.refreshAstraRoutingForSingleton(t.Context())
	require.NoError(t, err)
	require.True(t, refreshed.AccountScheduling)
	require.Equal(t, "same", refreshed.Revision)
	require.Equal(t, refreshed, cfg.AstraRouting(t.Context()))
}

func TestAstraSingletonPreparationRefreshAndBackoff(t *testing.T) {
	now := time.Now()
	settings := config.AstraRoutingSettings{Revision: "current", CookiePool: config.CodexGatewayPinConfig{Enabled: true, TargetAccountIDs: []int64{300}}}
	expiry := now.Add(5 * time.Minute)
	snapshot := AstraGatewayRuntime{Revision: "current", Targets: []AstraRouteStatus{{AccountID: 300, State: "ready", Reason: "target_probe_passed", ExpiresAt: &expiry}}}
	status := AstraSetupStatus{Revision: "current", State: "ready"}
	require.False(t, astraAutomaticSetupNeeded(settings, status, snapshot, now))
	expiry = now.Add(time.Minute)
	require.True(t, astraAutomaticSetupNeeded(settings, status, snapshot, now), "refresh before route expires without API traffic")
	status.State = "running"
	require.False(t, astraAutomaticSetupNeeded(settings, status, snapshot, now), "do not cancel active verification each tick")
	status.State = "failed"
	status.FinishedAt = &now
	require.False(t, astraAutomaticSetupNeeded(settings, status, snapshot, now))
	require.True(t, astraAutomaticSetupNeeded(settings, status, snapshot, now.Add(30*time.Second)))
	settings.Revision = "new"
	require.True(t, astraAutomaticSetupNeeded(settings, status, snapshot, now), "new save replaces old work immediately")
}
