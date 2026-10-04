package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const IntelligenceDrawingPrompt = "请直接返回完整的单文件HTML代码：使用内联SVG绘制山姆奥特曼{动作}在{场景}的2D动画。SVG必须设置viewBox，并用SVG或CSS实现动画；不要使用canvas、JavaScript、外部资源或Markdown代码块。你不需要任何测试"

type IntelligenceRuleConfig struct {
	ID      string             `json:"id"`
	Name    string             `json:"name"`
	Candy   *PelicanTestConfig `json:"candy"`
	Actions []string           `json:"actions"`
	Scenes  []string           `json:"scenes"`
}

type IntelligenceResultMetadata struct {
	Action       string `json:"action,omitempty"`
	Scene        string `json:"scene,omitempty"`
	FirstTokenMs *int64 `json:"first_token_ms,omitempty"`
	InputTokens  *int   `json:"input_tokens,omitempty"`
	OutputTokens *int   `json:"output_tokens,omitempty"`
}

type IntelligenceRuleInput struct {
	Name            string              `json:"name"`
	GroupIDs        []int64             `json:"group_ids"`
	ModelID         string              `json:"model_id"`
	ReasoningEffort string              `json:"reasoning_effort"`
	CronExpression  string              `json:"cron_expression"`
	Enabled         bool                `json:"enabled"`
	CandyPrompt     string              `json:"candy_prompt"`
	ExpectedAnswer  string              `json:"expected_answer"`
	Judge           *QualityJudgeConfig `json:"judge,omitempty"`
	DrawingPrompt   string              `json:"drawing_prompt"`
	Actions         []string            `json:"actions"`
	Scenes          []string            `json:"scenes"`
}

// Rules are transactionally materialized into the existing per-group schedules.
type IntelligenceRuleRepository interface {
	SaveIntelligenceRule(context.Context, string, []*PelicanGroupTestPlan) error
	DeleteIntelligenceRule(context.Context, string) error
}

func (s *PelicanGroupTestService) SaveIntelligenceRule(ctx context.Context, id string, input IntelligenceRuleInput) (string, error) {
	if id == "" {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		id = hex.EncodeToString(b[:])
	}
	if len(id) > 64 {
		return "", infraerrors.BadRequest("INVALID_INTELLIGENCE_RULE", "invalid rule id")
	}
	plans, err := s.intelligencePlans(ctx, id, input)
	if err != nil {
		return "", err
	}
	repo, ok := s.repo.(IntelligenceRuleRepository)
	if !ok {
		return "", fmt.Errorf("intelligence rule repository unavailable")
	}
	if err = repo.SaveIntelligenceRule(ctx, id, plans); err != nil {
		return "", err
	}
	return id, nil
}
func (s *PelicanGroupTestService) DeleteIntelligenceRule(ctx context.Context, id string) error {
	repo, ok := s.repo.(IntelligenceRuleRepository)
	if !ok {
		return fmt.Errorf("intelligence rule repository unavailable")
	}
	return repo.DeleteIntelligenceRule(ctx, id)
}

func (s *PelicanGroupTestService) intelligencePlans(ctx context.Context, id string, in IntelligenceRuleInput) ([]*PelicanGroupTestPlan, error) {
	invalid := func(message string) ([]*PelicanGroupTestPlan, error) {
		return nil, infraerrors.BadRequest("INVALID_INTELLIGENCE_RULE", message)
	}
	in.Name = strings.TrimSpace(in.Name)
	in.CandyPrompt = strings.TrimSpace(in.CandyPrompt)
	in.ExpectedAnswer = strings.TrimSpace(in.ExpectedAnswer)
	if in.Name == "" || len(in.Name) > 200 {
		return invalid("rule name is required (maximum 200 bytes)")
	}
	if len(in.GroupIDs) == 0 || len(in.GroupIDs) > 100 {
		return invalid("select 1–100 groups")
	}
	if in.CandyPrompt == "" || len(in.CandyPrompt) > 32000 || in.ExpectedAnswer == "" || len(in.ExpectedAnswer) > 4000 {
		return invalid("question and reference answer are required")
	}
	if in.Judge == nil && (in.CandyPrompt != strings.TrimSpace(CandyPrompt) || in.ExpectedAnswer != "21") {
		return invalid("custom questions or reference answers require a quality judge")
	}
	candy := &PelicanTestConfig{QuestionKind: "candy", Prompt: in.CandyPrompt, ReasoningEffort: normalizePelicanReasoningEffort(in.ReasoningEffort), ParallelCount: 1, Quality: &QualityPolicy{ExpectedAnswer: in.ExpectedAnswer, Judge: in.Judge, Action: QualityActionObserveOnly}}
	if err := validateQualityPolicy(&ScheduledTestPlan{PelicanConfig: candy}); err != nil {
		return invalid(err.Error())
	}
	if !strings.Contains(in.DrawingPrompt, "{动作}") || !strings.Contains(in.DrawingPrompt, "{场景}") {
		return invalid("drawing template must contain {动作} and {场景}")
	}
	validateChoices := func(items []string) bool {
		if len(items) == 0 || len(items) > 100 {
			return false
		}
		for _, v := range items {
			if strings.TrimSpace(v) == "" || len(v) > 200 || strings.ContainsAny(v, "{}\r\n") {
				return false
			}
		}
		return true
	}
	if !validateChoices(in.Actions) || !validateChoices(in.Scenes) {
		return invalid("provide 1–100 nonempty actions and scenes (maximum 200 bytes each)")
	}
	seen := map[int64]bool{}
	plans := make([]*PelicanGroupTestPlan, 0, len(in.GroupIDs))
	for _, gid := range in.GroupIDs {
		if seen[gid] {
			return invalid("duplicate group")
		}
		seen[gid] = true
		p, err := s.planFromInput(ctx, PelicanGroupTestPlanInput{GroupID: gid, ModelID: in.ModelID, ReasoningEffort: in.ReasoningEffort, CronExpression: in.CronExpression, Enabled: in.Enabled, Prompt: in.DrawingPrompt, ParallelCount: 1}, nil)
		if err != nil {
			return nil, err
		}
		p.PelicanConfig.Intelligence = &IntelligenceRuleConfig{ID: id, Name: in.Name, Candy: candy, Actions: append([]string(nil), in.Actions...), Scenes: append([]string(nil), in.Scenes...)}
		plans = append(plans, p)
	}
	return plans, nil
}

func intelligenceChoice(items []string) string {
	if len(items) == 0 {
		return ""
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(items))))
	if err != nil {
		return items[0]
	}
	return items[n.Int64()]
}

func (s *PelicanGroupTestService) intelligenceSamplePlans(plan *PelicanGroupTestPlan) []*PelicanGroupTestPlan {
	cfg := plan.PelicanConfig
	if cfg == nil || cfg.Intelligence == nil {
		return []*PelicanGroupTestPlan{plan}
	}
	rule := cfg.Intelligence
	candyPlan := *plan
	candy := *rule.Candy
	candy.Intelligence = nil
	candy.IntelligenceResult = &IntelligenceResultMetadata{}
	candy.ModelID = plan.ModelID
	candy.ReasoningEffort = cfg.ReasoningEffort
	candyPlan.PelicanConfig = &candy
	drawingPlan := *plan
	drawing := *cfg
	drawing.Intelligence = nil
	drawing.QuestionKind = "pelican"
	drawing.IntelligenceResult = &IntelligenceResultMetadata{Action: intelligenceChoice(rule.Actions), Scene: intelligenceChoice(rule.Scenes)}
	drawing.Prompt = strings.NewReplacer("{动作}", drawing.IntelligenceResult.Action, "{场景}", drawing.IntelligenceResult.Scene).Replace(cfg.Prompt)
	drawingPlan.PelicanConfig = &drawing
	return []*PelicanGroupTestPlan{&candyPlan, &drawingPlan}
}

func (s *PelicanGroupTestService) gradeIntelligence(ctx context.Context, accountID int64, cfg *PelicanTestConfig, r *ScheduledTestResult) {
	if cfg == nil || cfg.QuestionKind != "candy" || cfg.Quality == nil || r.Status != "success" {
		return
	}
	var judgment *QualityJudgment
	if cfg.Quality.Judge == nil && isBuiltinCandyPlan(cfg) && cfg.Quality.ExpectedAnswer == "21" {
		judgment = &QualityJudgment{Verdict: "incorrect", Reason: "builtin_candy"}
		if CandyAnswerCorrect(r.ResponseText) {
			judgment.Verdict = "correct"
		}
	} else if s.judgeQuality != nil {
		judgment = s.judgeQuality(ctx, accountID, cfg, r.ResponseText)
	}
	applyQualityJudgment(r, judgment)
}
