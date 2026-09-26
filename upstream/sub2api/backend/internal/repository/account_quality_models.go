package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Read only the native model configuration, never snapshot authentication secrets.
// ApplyQualityOutcome already holds the account row lock for the whole action.
func qualityModelAccount(ctx context.Context, tx *sql.Tx, id int64) (*service.Account, error) {
	a := &service.Account{}
	var mapping, extra []byte
	err := tx.QueryRowContext(ctx, `SELECT platform,type,COALESCE(credentials->'model_mapping','{}'::jsonb),COALESCE(extra,'{}'::jsonb) FROM accounts WHERE id=$1`, id).Scan(&a.Platform, &a.Type, &mapping, &extra)
	if err != nil {
		return nil, err
	}
	var models map[string]any
	if err = json.Unmarshal(mapping, &models); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(extra, &a.Extra); err != nil {
		return nil, err
	}
	a.Credentials = map[string]any{"model_mapping": models}
	return a, nil
}

func explicitQualityModels(a *service.Account) (map[string]any, bool) {
	if a.IsOpenAIPassthroughEnabled() {
		return nil, false
	}
	models, _ := a.Credentials["model_mapping"].(map[string]any)
	if len(models) == 0 {
		return nil, false
	}
	for key, value := range models {
		target, ok := value.(string)
		if !ok || strings.TrimSpace(target) == "" || strings.TrimSpace(key) == "" || strings.Contains(key, "*") {
			return nil, false
		}
	}
	return models, true
}

func removeQualityModels(a *service.Account, selected []string) (map[string]string, bool) {
	models, ok := explicitQualityModels(a)
	if !ok || len(selected) == 0 {
		return nil, false
	}
	removed := make(map[string]string, len(selected))
	next := make(map[string]any, len(models))
	for k, v := range models {
		next[k] = v
	}
	for _, model := range selected {
		target, exists := models[model].(string)
		if !exists {
			return nil, false
		}
		removed[model] = target
		delete(next, model)
	}
	// An empty mapping means unrestricted/default support in the native scheduler.
	if len(next) == 0 {
		return nil, false
	}
	candidate := *a
	candidate.Credentials = map[string]any{"model_mapping": next}
	for model := range removed {
		// Native aliases or platform defaults may reintroduce a deleted entry.
		if service.IsNativeModelSupportedByAccount(&candidate, model) {
			return nil, false
		}
	}
	a.Credentials = candidate.Credentials
	return removed, true
}

func restoreQualityModels(a *service.Account, removed map[string]string) bool {
	models, ok := explicitQualityModels(a)
	if !ok || len(removed) == 0 {
		return false
	}
	for model := range removed {
		if _, exists := models[model]; exists {
			return false
		}
	}
	for model, target := range removed {
		models[model] = target
	}
	return true
}

func writeQualityModels(ctx context.Context, tx *sql.Tx, id int64, a *service.Account) error {
	raw, err := json.Marshal(a.Credentials["model_mapping"])
	if err != nil {
		return err
	}
	// Patch only the native mapping. Credentials, groups and schedulable are preserved.
	_, err = tx.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(COALESCE(credentials,'{}'::jsonb),'{model_mapping}',$2::jsonb),updated_at=GREATEST(clock_timestamp(),updated_at+interval '1 microsecond') WHERE id=$1`, id, string(raw))
	return err
}
