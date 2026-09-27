package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"
)

const (
	ExcelBPSShadowRecoveryKey = "openai_excel_bps_shadow_recovery"
	ExcelBPSFallbackModelsKey = "openai_excel_bps_fallback_models"
	ExcelBPSRecoveryKey       = "openai_excel_bps_recovery"
)

type ExcelBPSRecoveryState struct {
	Active        bool      `json:"active"`
	Generation    string    `json:"generation"`
	DegradedAt    time.Time `json:"degraded_at"`
	TriggerStatus int       `json:"trigger_status"`
	Failures      int       `json:"failures"`
	NextProbeAt   time.Time `json:"next_probe_at"`
	LeaseToken    string    `json:"lease_token,omitempty"`
	LeaseUntil    time.Time `json:"lease_until,omitempty"`
	LastResult    string    `json:"last_result,omitempty"`
}

type AccountExcelBPSRecoveryRepository interface {
	DegradeExcelBPS(context.Context, *Account, int) (bool, error)
	ClaimDueExcelBPSRecoveries(context.Context, time.Time, int) ([]*Account, error)
	FinishExcelBPSRecovery(context.Context, *Account, bool, time.Time) (bool, error)
}

func (a *Account) ExcelBPSRecovery() ExcelBPSRecoveryState {
	var state ExcelBPSRecoveryState
	if a != nil {
		raw, _ := json.Marshal(a.Extra[ExcelBPSRecoveryKey])
		_ = json.Unmarshal(raw, &state)
	}
	return state
}
func (a *Account) IsExcelBPSDegraded() bool {
	return a.IsExcelBPSConfigured() && a.ExcelBPSRecovery().Active
}
func (a *Account) IsExcelBPSShadowRecoveryEnabled() bool {
	return a.IsExcelBPSConfigured() && a.Extra[ExcelBPSShadowRecoveryKey] == true
}
func ExcelBPSRecoveryDelay(failures int) time.Duration {
	if failures < 0 {
		failures = 0
	}
	if failures > 5 {
		failures = 5
	}
	return time.Duration(failures+1) * 5 * time.Minute
}
func IsExcelBPSDegradationStatus(status int) bool {
	return status == 403 || (status >= 500 && status <= 599)
}

// An explicitly configured fallback whitelist is also an identity mapping. It
// must not inherit BPS-only aliases or passthrough's allow-all short circuit.
func (a *Account) excelBPSFallbackMapping() (map[string]string, bool) {
	if !a.IsExcelBPSDegraded() {
		return nil, false
	}
	raw, exists := a.Extra[ExcelBPSFallbackModelsKey]
	if !exists {
		return nil, false
	}
	mapping := map[string]string{}
	switch list := raw.(type) {
	case []string:
		for _, model := range list {
			if model = strings.TrimSpace(model); model != "" {
				mapping[model] = model
			}
		}
	case []any:
		for _, value := range list {
			if model, ok := value.(string); ok && strings.TrimSpace(model) != "" {
				mapping[strings.TrimSpace(model)] = strings.TrimSpace(model)
			}
		}
	}
	return mapping, true
}

// MergeExcelBPSRecoveryExtra runs under the account row lock. Runtime state is
// never accepted from an admin payload; turning desired BPS off cancels it.
func MergeExcelBPSRecoveryExtra(extra, current map[string]any) map[string]any {
	extra = preserveExcelBPSFallbackModels(extra, current)
	delete(extra, ExcelBPSRecoveryKey)
	if extra["openai_excel_bps"] != true {
		delete(extra, ExcelBPSShadowRecoveryKey)
		delete(extra, ExcelBPSFallbackModelsKey)
	}
	if extra["openai_excel_bps"] == true {
		if state, ok := current[ExcelBPSRecoveryKey]; ok {
			if extra == nil {
				extra = map[string]any{}
			}
			extra[ExcelBPSRecoveryKey] = state
		}
	}
	return extra
}

// A full extra replacement may omit unchanged UI fields. Preserve the saved
// fallback whitelist, including while recovery is paused; patches stay patches.
func preserveExcelBPSFallbackModels(extra, current map[string]any) map[string]any {
	result := maps.Clone(extra)
	if result["openai_excel_bps"] == true {
		if _, supplied := result[ExcelBPSFallbackModelsKey]; !supplied {
			if saved, exists := current[ExcelBPSFallbackModelsKey]; exists {
				result[ExcelBPSFallbackModelsKey] = saved
			}
		}
	}
	return result
}

func ValidateExcelBPSRecoveryExtra(extra, current map[string]any) error {
	_, shadowChanged := extra[ExcelBPSShadowRecoveryKey]
	_, modelsChanged := extra[ExcelBPSFallbackModelsKey]
	_, desiredChanged := extra["openai_excel_bps"]
	if !shadowChanged && !modelsChanged && !desiredChanged {
		return nil
	}
	merged := maps.Clone(current)
	if merged == nil {
		merged = map[string]any{}
	}
	for key, value := range extra {
		merged[key] = value
	}
	if value, ok := extra[ExcelBPSShadowRecoveryKey]; ok {
		if _, ok = value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", ExcelBPSShadowRecoveryKey)
		}
	}
	if desired, explicit := extra["openai_excel_bps"].(bool); explicit && !desired {
		return nil
	}
	if _, exists := extra[ExcelBPSFallbackModelsKey]; exists || (extra[ExcelBPSShadowRecoveryKey] == true && current[ExcelBPSShadowRecoveryKey] != true) {
		raw, _ := json.Marshal(merged[ExcelBPSFallbackModelsKey])
		var models []string
		if json.Unmarshal(raw, &models) != nil {
			return fmt.Errorf("fallback models must be a string list")
		}
		for _, model := range models {
			if strings.TrimSpace(model) == "" {
				return fmt.Errorf("fallback models must not contain empty names")
			}
		}
		if merged[ExcelBPSShadowRecoveryKey] == true && len(models) == 0 {
			return fmt.Errorf("shadow recovery requires at least one fallback model")
		}
	}

	if merged[ExcelBPSShadowRecoveryKey] == true && merged["openai_excel_bps"] != true {
		return fmt.Errorf("shadow recovery requires configured BPS")
	}
	return nil
}

type excelBPSShadowProbeKey struct{}

func isExcelBPSShadowProbe(ctx context.Context) bool {
	return ctx.Value(excelBPSShadowProbeKey{}) == true
}

// ExcelBPSRecoverySnapshotMatches ignores observational quota/usage updates but
// rejects changed credentials, routing settings or ownership before probing.
func ExcelBPSRecoverySnapshotMatches(a, b *Account) bool {
	if a == nil || b == nil || !b.IsSchedulableAt(time.Now()) || !b.IsExcelBPSShadowRecoveryEnabled() {
		return false
	}
	snapshot := func(a *Account) string {
		extra := map[string]any{}
		for key, value := range a.Extra {
			if strings.HasPrefix(key, "openai_excel_bps") {
				extra[key] = value
			}
		}
		raw, _ := json.Marshal([]any{a.Credentials, extra, a.ProxyID, ExcelBPSProxyTransportFingerprint(a.Proxy)})
		return string(raw)
	}
	return snapshot(a) == snapshot(b)
}

type excelBPSShadowProbeCheckKey struct{}

// ExcelBPSProxyTransportFingerprint deliberately excludes labels, warning
// preferences and observation timestamps. Never log the source credentials.
func ExcelBPSProxyTransportFingerprint(p *Proxy) string {
	if p == nil {
		return ""
	}
	var expiry any
	if p.ExpiresAt != nil {
		expiry = p.ExpiresAt.UTC()
	}
	raw, _ := json.Marshal([]any{p.ID, p.Protocol, p.Host, p.Port, p.Username, p.Password, p.Status, expiry, p.FallbackMode, p.BackupProxyID})
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
