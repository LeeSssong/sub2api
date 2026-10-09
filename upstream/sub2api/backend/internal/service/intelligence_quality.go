package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type IntelligenceQualitySource struct {
	GroupID    int64 `json:"group_id"`
	TemplateID int64 `json:"template_id"`
}
type IntelligenceQualityRepository interface {
	ResolveIntelligenceQualitySource(context.Context, int64, int64, []string) (*QualityRuleTemplate, error)
	SnapshotIntelligenceQuality(context.Context, *PelicanGroupTestPlan, time.Time) error
}

func normalizeIntelligenceModels(models []string) ([]string, error) {
	if len(models) == 0 {
		return []string{"gpt-6-astra", "gpt-6.1-sol"}, nil
	}
	if len(models) > 10 {
		return nil, fmt.Errorf("select at most ten candy models")
	}
	seen := map[string]bool{}
	out := []string{}
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" || len(m) > 200 || seen[m] {
			return nil, fmt.Errorf("invalid or duplicate candy model")
		}
		seen[m] = true
		out = append(out, m)
	}
	return out, nil
}

// Bound lookup to the dashboard retention window; a missing interval yields no sample.
func IntelligenceQualityWindow(expression string, slot time.Time) (time.Time, time.Time, error) {
	schedule, err := scheduledTestCronParser.Parse(expression)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	var start, end time.Time
	for next := schedule.Next(slot.Add(-72 * time.Hour)); !next.After(slot); next = schedule.Next(next) {
		start = end
		end = next
	}
	if start.IsZero() || end.IsZero() {
		return start, end, fmt.Errorf("no recent complete quality interval")
	}
	return start, end, nil
}
