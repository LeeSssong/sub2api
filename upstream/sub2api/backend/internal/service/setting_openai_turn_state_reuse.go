package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	OpenAITurnStateMissNone          = "none"
	OpenAITurnStateMissRebindGroup   = "rebind_group"
	OpenAITurnStateMissUnbindGroups  = "unbind_groups"
	OpenAITurnStateMissUnschedulable = "unschedulable"
	OpenAITurnStateRecoveredNone     = "none"
	OpenAITurnStateRecoveredRebind   = "rebind_group"
	OpenAITurnStateRecoveredRestore  = "restore_schedulable"
)

type OpenAITurnStateReuseSettings struct {
	Enabled                bool     `json:"enabled"`
	HarvestModel           string   `json:"harvest_model"`
	HarvestProxyURLs       []string `json:"harvest_proxy_urls"`
	HarvestUseProxyPool    bool     `json:"harvest_use_proxy_pool"`
	MissAction             string   `json:"miss_action"`
	MissTargetGroupID      *int64   `json:"miss_target_group_id,omitempty"`
	RecoveredAction        string   `json:"recovered_action"`
	RecoveredTargetGroupID *int64   `json:"recovered_target_group_id,omitempty"`
	InjectCompact          bool     `json:"inject_compact"`
}

func DefaultOpenAITurnStateReuseSettings() *OpenAITurnStateReuseSettings {
	return &OpenAITurnStateReuseSettings{HarvestModel: OpenAITurnStateHarvestModel, HarvestProxyURLs: []string{}, HarvestUseProxyPool: true, MissAction: OpenAITurnStateMissNone, RecoveredAction: OpenAITurnStateRecoveredNone}
}

func NormalizeOpenAITurnStateReuseSettings(in *OpenAITurnStateReuseSettings) (*OpenAITurnStateReuseSettings, error) {
	if in == nil {
		return nil, errors.New("settings cannot be nil")
	}
	out := *in
	out.HarvestModel = OpenAITurnStateHarvestModel
	out.InjectCompact = false
	if out.HarvestProxyURLs == nil {
		out.HarvestProxyURLs = []string{}
	}
	for i := range out.HarvestProxyURLs {
		out.HarvestProxyURLs[i] = strings.TrimSpace(out.HarvestProxyURLs[i])
	}
	switch out.MissAction {
	case OpenAITurnStateMissNone, OpenAITurnStateMissUnbindGroups, OpenAITurnStateMissUnschedulable:
		out.MissTargetGroupID = nil
	case OpenAITurnStateMissRebindGroup:
		if out.MissTargetGroupID == nil || *out.MissTargetGroupID <= 0 {
			return nil, errors.New("miss_target_group_id is required for rebind_group")
		}
	default:
		return nil, errors.New("invalid miss_action")
	}
	switch out.RecoveredAction {
	case OpenAITurnStateRecoveredNone, OpenAITurnStateRecoveredRestore:
		out.RecoveredTargetGroupID = nil
	case OpenAITurnStateRecoveredRebind:
		if out.RecoveredTargetGroupID == nil || *out.RecoveredTargetGroupID <= 0 {
			return nil, errors.New("recovered_target_group_id is required for rebind_group")
		}
	default:
		return nil, errors.New("invalid recovered_action")
	}
	return &out, nil
}

func (s *SettingService) GetOpenAITurnStateReuseSettings(ctx context.Context) (*OpenAITurnStateReuseSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyOpenAITurnStateReuseSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultOpenAITurnStateReuseSettings(), nil
		}
		return nil, fmt.Errorf("get OpenAI turn-state reuse settings: %w", err)
	}
	settings := DefaultOpenAITurnStateReuseSettings()
	if strings.TrimSpace(value) == "" {
		return settings, nil
	}
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		return DefaultOpenAITurnStateReuseSettings(), nil
	}
	return NormalizeOpenAITurnStateReuseSettings(settings)
}

func (s *SettingService) SetOpenAITurnStateReuseSettings(ctx context.Context, settings *OpenAITurnStateReuseSettings) error {
	normalized, err := NormalizeOpenAITurnStateReuseSettings(settings)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	return s.settingRepo.Set(ctx, SettingKeyOpenAITurnStateReuseSettings, string(raw))
}
