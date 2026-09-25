package service

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestcapture"
)

const (
	SettingKeyRequestCaptureEnabled       = "request_capture_enabled"
	SettingKeyRequestCaptureQuotaMiB      = "request_capture_quota_mib"
	SettingKeyRequestCaptureRetentionDays = "request_capture_retention_days"
)

func (s *SystemSettings) requestCaptureConfig() requestcapture.Config {
	c := requestcapture.Config{Enabled: s.RequestCaptureEnabled, QuotaMiB: s.RequestCaptureQuotaMiB, RetentionDays: s.RequestCaptureRetentionDays}
	if c.QuotaMiB == 0 {
		c.QuotaMiB = 1024
	}
	if c.RetentionDays == 0 {
		c.RetentionDays = 7
	}
	return c
}
func ProvideRequestCaptureManager(db *sql.DB, settings *SettingService, cfg *config.Config) (*requestcapture.Manager, error) {
	if !shouldStartRequestLocal(cfg) {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	config, err := settings.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "./data"
	}
	dir, err = requestCaptureDirectory(dir)
	if err != nil {
		return nil, err
	}
	// One owner per persistent slot. Never recover files while a live process still owns them.
	owner, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var acquired bool
	if err = owner.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1, 0))", "request-capture:"+dir).Scan(&acquired); err != nil || !acquired {
		_ = owner.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("request capture slot already has an active owner")
	}
	release := func() {
		releaseCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_, unlockErr := owner.ExecContext(releaseCtx, "SELECT pg_advisory_unlock(hashtextextended($1, 0))", "request-capture:"+dir)
		if unlockErr != nil {
			_ = owner.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = owner.Close()
	}
	manager, err := requestcapture.New(&requestcapture.SQLStore{DB: db}, dir, config.requestCaptureConfig())
	if err != nil {
		release()
		return nil, err
	}
	watchCtx, stopWatch := context.WithCancel(context.Background())
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-watchCtx.Done():
				return
			case <-ticker.C:
				refreshRequestCaptureConfig(watchCtx, manager, settings.settingRepo)
			}
		}
	}()
	manager.SetRelease(func() { stopWatch(); <-watchDone; release() })
	settings.requestCapture = manager
	return manager, nil
}

func requestCaptureDirectory(dataDir string) (string, error) {
	slot := strings.TrimSpace(os.Getenv("SUB2API_CONTAINER_SLOT"))
	if slot == "" {
		slot = "standalone"
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`).MatchString(slot) {
		return "", fmt.Errorf("invalid request capture slot")
	}
	return filepath.Join(dataDir, "request-captures", slot), nil
}

type requestCaptureSettingsReader interface {
	GetMultiple(context.Context, []string) (map[string]string, error)
}

func refreshRequestCaptureConfig(ctx context.Context, manager *requestcapture.Manager, repo requestCaptureSettingsReader) {
	query, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	values, err := repo.GetMultiple(query, []string{SettingKeyRequestCaptureEnabled, SettingKeyRequestCaptureQuotaMiB, SettingKeyRequestCaptureRetentionDays})
	c := manager.Config()
	if err != nil {
		c.Enabled = false
		manager.ApplyConfig(c)
		return
	}
	next := requestcapture.Config{Enabled: values[SettingKeyRequestCaptureEnabled] == "true", QuotaMiB: 1024, RetentionDays: 7}
	if raw := values[SettingKeyRequestCaptureQuotaMiB]; raw != "" {
		next.QuotaMiB, _ = strconv.ParseInt(raw, 10, 64)
	}
	if raw := values[SettingKeyRequestCaptureRetentionDays]; raw != "" {
		next.RetentionDays, _ = strconv.Atoi(raw)
	}
	if next.Validate() != nil {
		c.Enabled = false
		manager.ApplyConfig(c)
		return
	}
	manager.ApplyConfig(next)
}
