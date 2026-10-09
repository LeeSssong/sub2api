package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *pelicanGroupTestRepository) ResolveIntelligenceQualitySource(ctx context.Context, groupID, templateID int64, models []string) (*service.QualityRuleTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,model_id,cron_expression,pelican_config,updated_at FROM quality_rule_templates WHERE enabled AND account_filter->>'group'=$1 AND ($2::bigint=0 OR id=$2) ORDER BY id`, strconv.FormatInt(groupID, 10), templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var matches []*service.QualityRuleTemplate
	for rows.Next() {
		t := &service.QualityRuleTemplate{}
		var raw []byte
		if err = rows.Scan(&t.ID, &t.ModelID, &t.CronExpression, &raw, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &t.PelicanConfig); err != nil {
			return nil, err
		}
		if t.PelicanConfig == nil || t.PelicanConfig.QuestionKind != "candy" || t.PelicanConfig.Quality == nil {
			continue
		}
		available := map[string]bool{}
		if len(t.PelicanConfig.ModelIDs) == 0 {
			available[t.ModelID] = true
		}
		for _, m := range t.PelicanConfig.ModelIDs {
			available[m] = true
		}
		valid := true
		for _, m := range models {
			if !available[m] {
				valid = false
			}
		}
		if valid {
			matches = append(matches, t)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("quality source missing or ambiguous for group %d; select one enabled candy template with every model", groupID)
	}
	return matches[0], nil
}

// The advisory lock and stored display timestamp make selection durable across replicas
// and restarts, without changing the existing schema or charging a second test.
func (r *pelicanGroupTestRepository) SnapshotIntelligenceQuality(ctx context.Context, plan *service.PelicanGroupTestPlan, slot time.Time) error {
	cfg := plan.PelicanConfig.Intelligence
	models := cfg.CandyModels
	if len(models) == 0 {
		models = []string{"gpt-6-astra", "gpt-6.1-sol"}
	}
	source, err := r.ResolveIntelligenceQualitySource(ctx, plan.GroupID, cfg.QualityTemplateID, models)
	if err != nil {
		return err
	}
	expected := len(source.PelicanConfig.ModelIDs)
	if expected == 0 {
		expected = 1
	}
	parallel := source.PelicanConfig.ParallelCount
	if parallel < 1 {
		parallel = 1
	}
	modelCount := expected
	expected *= parallel
	start, end, err := service.IntelligenceQualityWindow(source.CronExpression, slot)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Keep the source binding stable while copying; template edits/deletion wait.
	var lockedID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM quality_rule_templates WHERE id=$1 AND enabled AND account_filter->>'group'=$2 AND cron_expression=$3 AND updated_at=$4 FOR SHARE`, source.ID, strconv.FormatInt(plan.GroupID, 10), source.CronExpression, source.UpdatedAt).Scan(&lockedID); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("intelligence-quality:%d:%s", plan.ID, slot.UTC().Format(time.RFC3339))); err != nil {
		return err
	}
	for _, model := range models {
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pelican_group_test_results WHERE plan_id=$1 AND started_at=$2 AND pelican_config->'intelligence_result'->>'source'='quality_ops' AND pelican_config->>'model_id'=$3)`, plan.ID, slot, model).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		var id int64
		var status, response, message string
		var latency int64
		var began, finished time.Time
		var raw, judgment []byte
		err = tx.QueryRowContext(ctx, `WITH rounds AS (
   SELECT l.account_id,r.plan_id,COALESCE(NULLIF(r.quality_round_id,''),r.id::text) round_id,MAX(r.finished_at) completed
   FROM quality_rule_template_accounts l JOIN scheduled_test_results r ON r.plan_id=l.plan_id
   WHERE l.template_id=$1 AND l.tested_group_id=$2
   AND COALESCE(r.pelican_config->>'trigger_source','scheduled')='scheduled'
   GROUP BY l.account_id,r.plan_id,COALESCE(NULLIF(r.quality_round_id,''),r.id::text)
   HAVING MIN(r.started_at)>=$3 AND MAX(r.finished_at)<=$4 AND COUNT(*)=COUNT(r.finished_at)
   AND MAX(r.finished_at)>=$3 AND COUNT(*) >= $6 AND COUNT(DISTINCT r.pelican_config->>'model_id') >= $7
  ), latest AS (
   SELECT *,row_number() OVER(PARTITION BY account_id ORDER BY completed DESC,round_id DESC) pos FROM rounds
  ), eligible AS (
   SELECT l.account_id,r.* FROM latest l JOIN scheduled_test_results r ON r.plan_id=l.plan_id AND COALESCE(NULLIF(r.quality_round_id,''),r.id::text)=l.round_id
   WHERE l.pos=1 AND r.status<>'skipped' AND r.pelican_config->>'model_id'=$5 AND r.pelican_config->>'question_kind'='candy'
  ), chosen AS (SELECT account_id FROM eligible GROUP BY account_id ORDER BY random() LIMIT 1)
  SELECT r.id,r.status,r.response_text,r.error_message,r.latency_ms,r.started_at,r.finished_at,r.pelican_config,r.quality_judgment
  FROM eligible r JOIN chosen c USING(account_id) ORDER BY random() LIMIT 1`, source.ID, plan.GroupID, start, end, model, expected, modelCount).Scan(&id, &status, &response, &message, &latency, &began, &finished, &raw, &judgment)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		var original service.PelicanTestConfig
		if err = json.Unmarshal(raw, &original); err != nil {
			return err
		}
		verdict := "unknown"
		var j service.QualityJudgment
		if len(judgment) > 0 && string(judgment) != "null" {
			if err = json.Unmarshal(judgment, &j); err != nil {
				return err
			}
			if j.Verdict == "correct" {
				verdict = "passed"
			} else if j.Verdict == "incorrect" {
				verdict = "incorrect"
			}
		}
		if verdict == "unknown" && status != "success" && (len(judgment) == 0 || string(judgment) == "null") && message != "judge_unknown" {
			verdict = "abnormal"
		}
		snapshot := &service.PelicanTestConfig{QuestionKind: "candy", ModelID: model, ReasoningEffort: original.ReasoningEffort, Prompt: original.Prompt, IntelligenceResult: &service.IntelligenceResultMetadata{Source: "quality_ops", SourceResultID: id, SourceTemplateID: source.ID, SourceStartedAt: &began, SourceFinishedAt: &finished, Verdict: verdict}}
		if original.Quality != nil {
			snapshot.Quality = &service.QualityPolicy{ExpectedAnswer: original.Quality.ExpectedAnswer, Action: service.QualityActionObserveOnly}
		}
		// Error bodies and private judge configuration are deliberately not copied.
		safeError := ""
		if verdict == "abnormal" {
			safeError = "request_failed"
		}
		if verdict == "incorrect" {
			safeError = "answer_mismatch"
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO pelican_group_test_results(plan_id,account_name,attempts,status,response_text,error_message,latency_ms,pelican_config,started_at,finished_at,cost_usd,cost_incomplete) VALUES($1,'','[]',$2,$3,$4,$5,$6,$7,$7,0,false)`, plan.ID, status, response, safeError, latency, marshalPelicanConfig(snapshot), slot)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// BackfillIntelligenceQuality copies only still-retained source history on first use.
// It never runs a drawing or an upstream quality request. Each slot uses the same
// durable sampling transaction as normal display collection.
func (r *pelicanGroupTestRepository) BackfillIntelligenceQuality(ctx context.Context, plan *service.PelicanGroupTestPlan, now time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pelican_group_test_results WHERE plan_id=$1 AND pelican_config->'intelligence_result'->>'source'='quality_ops')`, plan.ID).Scan(&exists); err != nil || exists {
		return err
	}
	cfg := plan.PelicanConfig.Intelligence
	models := cfg.CandyModels
	if len(models) == 0 {
		models = []string{"gpt-6-astra", "gpt-6.1-sol"}
	}
	source, err := r.ResolveIntelligenceQualitySource(ctx, plan.GroupID, cfg.QualityTemplateID, models)
	if err != nil {
		return err
	}
	var earliest sql.NullTime
	if err = r.db.QueryRowContext(ctx, `SELECT MIN(r.started_at) FROM quality_rule_template_accounts l JOIN scheduled_test_results r ON r.plan_id=l.plan_id WHERE l.template_id=$1 AND l.tested_group_id=$2 AND r.started_at >= $3 AND r.finished_at <= $4 AND r.status<>'skipped'`, source.ID, plan.GroupID, now.Add(-72*time.Hour), now).Scan(&earliest); err != nil {
		return err
	}
	if !earliest.Valid {
		return nil
	}
	for slot := earliest.Time.Truncate(30 * time.Minute).Add(30 * time.Minute); !slot.After(now.Truncate(30 * time.Minute)); slot = slot.Add(30 * time.Minute) {
		if err = r.SnapshotIntelligenceQuality(ctx, plan, slot); err != nil {
			return err
		}
	}
	return nil
}
