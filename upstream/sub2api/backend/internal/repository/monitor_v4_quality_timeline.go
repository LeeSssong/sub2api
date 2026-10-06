package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type gradedCandySample struct {
	result   service.ScheduledTestResult
	answered bool
}
type qualityTimelineRoundKey struct {
	groupID, planID int64
	roundID         string
}
type qualityTimelineRound struct {
	finishedAt time.Time
	samples    []gradedCandySample
}

// A round has a meaningful score only when every configured model/sample has
// a completed answer and a correct/incorrect judgment. Partial saves, skipped
// models, transport errors and judge failures must not become 0% or degradation.
func classifyGradedCandyRound(samples []gradedCandySample) (valid, degraded bool) {
	if len(samples) == 0 || samples[0].result.PelicanConfig == nil {
		return false, false
	}
	cfg := samples[0].result.PelicanConfig
	models := cfg.ModelIDs
	if len(models) == 0 {
		models = []string{cfg.ModelID}
	}
	if cfg.ParallelCount < 1 || len(samples) != len(models)*cfg.ParallelCount {
		return false, false
	}
	counts := make(map[string]int, len(models))
	for _, model := range models {
		if model == "" {
			return false, false
		}
		if _, exists := counts[model]; exists {
			return false, false
		}
		counts[model] = 0
	}
	for _, sample := range samples {
		r := sample.result
		c := r.PelicanConfig
		if !sample.answered || c == nil || c.QuestionKind != "candy" || c.Quality == nil || c.ParallelCount != cfg.ParallelCount || r.QualityJudgment == nil {
			return false, false
		}
		if len(c.ModelIDs) != len(cfg.ModelIDs) {
			return false, false
		}
		for i, model := range c.ModelIDs {
			if model != cfg.ModelIDs[i] {
				return false, false
			}
		}
		if _, exists := counts[c.ModelID]; !exists {
			return false, false
		}
		counts[c.ModelID]++
		switch r.QualityJudgment.Verdict {
		case "correct":
			if r.Status != "success" || r.ErrorMessage != "" {
				return false, false
			}
		case "incorrect":
			if r.Status != "failed" || r.ErrorMessage != "answer_mismatch" {
				return false, false
			}
			degraded = true
		default:
			return false, false
		}
	}
	for _, count := range counts {
		if count != cfg.ParallelCount {
			return false, false
		}
	}
	return true, degraded
}

func applyGradedCandyRounds(points []service.MonitorV4TimelinePoint, rounds map[qualityTimelineRoundKey]*qualityTimelineRound) {
	for key, round := range rounds {
		valid, degraded := classifyGradedCandyRound(round.samples)
		if !valid {
			continue
		}
		for i := range points {
			p := &points[i]
			if p.GroupID == key.groupID && !round.finishedAt.Before(p.Start) && round.finishedAt.Before(p.End) {
				p.GradedRoundCount++
				if degraded {
					p.SuspectedDegradedRoundCount++
				}
				break
			}
		}
	}
}

// Scope comes from the link's immutable tested-group snapshot, never from the
// template's editable filter, the judge's group or today's account membership.
// Ambiguous legacy links remain NULL. Separate reads preserve request counts.
const monitorV4GradedCandySQL = `WITH scoped_plans AS (
 SELECT DISTINCT g.group_id, link.plan_id
 FROM unnest($1::bigint[]) AS g(group_id)
 JOIN quality_rule_template_accounts link ON link.tested_group_id=g.group_id
 WHERE link.plan_id IS NOT NULL
), selected_rounds AS (
 SELECT DISTINCT scope.group_id, r.plan_id, r.quality_round_id
 FROM scoped_plans scope JOIN scheduled_test_results r ON r.plan_id=scope.plan_id
 WHERE r.finished_at >= $2::timestamptz AND r.finished_at < $3::timestamptz
   AND r.quality_round_id IS NOT NULL AND r.quality_round_id<>''
   AND r.pelican_config->>'question_kind'='candy'
   AND r.pelican_config->'quality' IS NOT NULL
)
 SELECT scope.group_id,r.plan_id,r.quality_round_id,r.finished_at,
        r.status,r.error_message,r.pelican_config,r.quality_judgment,
        COALESCE(length(btrim(r.response_text))>0,FALSE)
 FROM selected_rounds scope JOIN scheduled_test_results r
 ON r.plan_id=scope.plan_id AND r.quality_round_id=scope.quality_round_id
 ORDER BY scope.group_id,r.plan_id,r.quality_round_id,r.id`

func (r *accountMonitorRepository) readGradedCandyRounds(ctx context.Context, ids []int64, start, end time.Time, points []service.MonitorV4TimelinePoint) error {
	rows, err := r.db.QueryContext(ctx, monitorV4GradedCandySQL, pq.Array(ids), start.UTC(), end.UTC())
	if err != nil {
		return err
	}
	defer rows.Close()
	rounds := map[qualityTimelineRoundKey]*qualityTimelineRound{}
	for rows.Next() {
		var key qualityTimelineRoundKey
		var sample gradedCandySample
		var cfg, judgment []byte
		if err := rows.Scan(&key.groupID, &key.planID, &key.roundID, &sample.result.FinishedAt, &sample.result.Status, &sample.result.ErrorMessage, &cfg, &judgment, &sample.answered); err != nil {
			return err
		}
		if len(cfg) > 0 {
			if err := json.Unmarshal(cfg, &sample.result.PelicanConfig); err != nil {
				return err
			}
		}
		if len(judgment) > 0 {
			if err := json.Unmarshal(judgment, &sample.result.QualityJudgment); err != nil {
				return err
			}
		}
		round := rounds[key]
		if round == nil {
			round = &qualityTimelineRound{}
			rounds[key] = round
		}
		if sample.result.FinishedAt.After(round.finishedAt) {
			round.finishedAt = sample.result.FinishedAt
		}
		round.samples = append(round.samples, sample)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	applyGradedCandyRounds(points, rounds)
	return nil
}
