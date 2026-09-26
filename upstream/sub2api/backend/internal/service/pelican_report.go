package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"sort"
	"strings"
	"time"
)

var ErrPelicanReportNotFound = infraerrors.NotFound("PELICAN_REPORT_NOT_FOUND", "report group not found")

// PelicanReportExecutionMeta is captured by a successful scheduler claim, never inferred.
type PelicanReportExecutionMeta struct {
	ID, SharedRoundID, Timezone string
	ScheduledFor                *time.Time
	ExpectedCount               int
}
type PelicanReportFact struct {
	ResultID                                                    int64
	Kind, ModelID, ExecutionID, SharedRoundID, Status, Judgment string
	ExpectedCount                                               int
	ScheduledFor, StartedAt, CompletedAt                        *time.Time
	ObservedAt                                                  time.Time
}
type PelicanReportGroup struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Platform       string   `json:"platform"`
	RateMultiplier *float64 `json:"rate_multiplier"`
}
type PelicanReportResult struct {
	ResultID      int64      `json:"result_id"`
	ExecutionID   *string    `json:"execution_id"`
	SharedRoundID *string    `json:"shared_round_id"`
	ScheduledFor  *time.Time `json:"scheduled_for"`
	Status        string     `json:"status"`
	Judgment      string     `json:"judgment"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	LatencyMs     *int64     `json:"latency_ms"`
}
type PelicanReportExecution struct {
	ExecutionID    *string               `json:"execution_id"`
	SharedRoundID  *string               `json:"shared_round_id"`
	ScheduledFor   *time.Time            `json:"scheduled_for"`
	Status         string                `json:"status"`
	ExpectedCount  *int                  `json:"expected_count"`
	CompletedCount int                   `json:"completed_count"`
	StartedAt      *time.Time            `json:"started_at"`
	CompletedAt    *time.Time            `json:"completed_at"`
	LatencyMs      *int64                `json:"latency_ms"`
	Results        []PelicanReportResult `json:"results"`
}
type PelicanReportStats struct {
	SuccessCount   int64    `json:"success_count"`
	TotalCount     int64    `json:"total_count"`
	FailureCount   int64    `json:"failure_count"`
	UngradedCount  int64    `json:"ungraded_count"`
	ObservedCount  int64    `json:"observed_count"`
	SuccessRate    *float64 `json:"success_rate"`
	TimedCount     int64    `json:"timed_count"`
	AvgLatencyMs   *float64 `json:"avg_latency_ms"`
	ExecutionCount *int64   `json:"execution_count"`
}

const PelicanReportHistoryLimit = 240

type PelicanReportStatistics struct {
	Candy   PelicanReportStats `json:"candy"`
	Pelican PelicanReportStats `json:"pelican"`
}

type PelicanReportHistoryMeta struct {
	TotalCount    int64 `json:"total_count"`
	ReturnedCount int   `json:"returned_count"`
	Limit         int   `json:"limit"`
}

type PelicanReportArtwork struct {
	ID              int64     `json:"id"`
	SourceResultID  int64     `json:"source_result_id"`
	ExecutionID     *string   `json:"execution_id"`
	GroupID         int64     `json:"group_id"`
	ModelID         string    `json:"model_id"`
	ReasoningEffort string    `json:"reasoning_effort"`
	GeneratedAt     time.Time `json:"generated_at"`
	ResponseText    string    `json:"response_text"`
}
type PelicanReportRound struct {
	RoundID            string    `json:"round_id"`
	ScheduledFor       time.Time `json:"scheduled_for"`
	StartedAt          time.Time `json:"started_at"`
	CompletedAt        time.Time `json:"completed_at"`
	LatencyMs          int64     `json:"latency_ms"`
	CandyExecutionID   string    `json:"candy_execution_id"`
	PelicanExecutionID string    `json:"pelican_execution_id"`
}
type PelicanReportView struct {
	SchemaVersion int                `json:"schema_version"`
	AsOf          time.Time          `json:"as_of"`
	SnapshotID    string             `json:"snapshot_id"`
	Group         PelicanReportGroup `json:"group"`
	ModelID       *string            `json:"model_id"`
	Window        struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
	} `json:"window"`
	Statistics PelicanReportStatistics `json:"statistics"`
	History    struct {
		Candy   []PelicanReportResult `json:"candy"`
		Pelican []PelicanReportResult `json:"pelican"`
	} `json:"history"`
	HistoryMeta struct {
		Candy   PelicanReportHistoryMeta `json:"candy"`
		Pelican PelicanReportHistoryMeta `json:"pelican"`
	} `json:"history_meta"`
	Latest struct {
		Candy   *PelicanReportExecution `json:"candy"`
		Pelican *PelicanReportExecution `json:"pelican"`
	} `json:"latest"`
	CurrentRound *PelicanReportRound   `json:"current_round"`
	Artwork      *PelicanReportArtwork `json:"artwork"`
}

// Data is one repeatable-read snapshot without credentials, account identities or error bodies.
type PelicanReportData struct {
	Group      PelicanReportGroup
	ModelID    string
	Facts      []PelicanReportFact
	Artwork    *PelicanReportArtwork
	Statistics *PelicanReportStatistics
}
type PelicanReportRepository interface {
	ReadReport(context.Context, int64, string, []int64, int, time.Time, time.Time, time.Time) (*PelicanReportData, error)
}

func (s *PelicanShowcaseService) Report(ctx context.Context, groupID int64, model string, now time.Time) (*PelicanReportView, error) {
	runtime, err := s.settings.GetPelicanShowcaseRuntime(ctx)
	if err != nil {
		return nil, err
	}
	if !runtime.Enabled || groupID <= 0 {
		return nil, ErrPelicanReportNotFound
	}
	allowed := false
	for _, id := range runtime.Config.GroupIDs {
		if id == groupID {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrPelicanReportNotFound
	}
	repo, ok := s.repo.(PelicanReportRepository)
	if !ok {
		return nil, fmt.Errorf("report repository unavailable")
	}
	now = now.UTC()
	data, err := repo.ReadReport(ctx, groupID, model, runtime.Config.GroupIDs, runtime.Config.MaxItems, runtime.Config.retentionCutoff(now), now.Add(-24*time.Hour), now)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, ErrPelicanReportNotFound
	}
	return buildPelicanReport(data, now), nil
}

// Classify distinguishes a completed request from an actual candy answer judgment.
func ClassifyPelicanReportResult(result *ScheduledTestResult) (kind, judgment, status string) {
	if result == nil || result.PelicanConfig == nil || result.PelicanConfig.Quality != nil || (result.Status != "success" && result.Status != "failed") {
		return
	}
	cfg := result.PelicanConfig
	status = result.Status
	if isBuiltinCandyPlan(cfg) {
		kind, judgment = "candy", "builtin_candy"
		if status == "success" && strings.TrimSpace(result.ResponseText) != "21" {
			status = "failed"
		}
	} else if cfg.QuestionKind == "candy" {
		kind, judgment = "candy", "ungraded_candy"
		if status == "success" {
			status = "ungraded"
		}
	} else if cfg.QuestionKind == "" || cfg.QuestionKind == "pelican" {
		kind, judgment = "pelican", "drawing"
		if status == "success" && !pelicanHTMLPattern.MatchString(result.ResponseText) {
			status = "failed"
		}
	}
	return
}

// PelicanReportPairFingerprint binds a slot to the exact current peer versions.
// Natural claims advance next_run_at without editing updated_at; this lets the
// second peer consume the slot but invalidates it after any plan edit.
func PelicanReportPairFingerprint(plan *ScheduledTestPlan, peers []*ScheduledTestPlan, zone string) string {
	if plan == nil || plan.PelicanConfig == nil || plan.PelicanConfig.ReportPairKey == "" || len(peers) != 2 || zone == "" || strings.Contains(plan.CronExpression, "@every") {
		return ""
	}
	kinds := map[string]bool{}
	members := make([]*ScheduledTestPlan, 0, 2)
	own := false
	for _, p := range peers {
		if p == nil || !p.Enabled || p.ID <= 0 || p.AccountID != plan.AccountID || p.PelicanConfig == nil || p.PelicanConfig.Quality != nil || p.PelicanConfig.ReportPairKey != plan.PelicanConfig.ReportPairKey || p.ModelID != plan.ModelID || p.CronExpression != plan.CronExpression {
			return ""
		}
		k, _, _ := ClassifyPelicanReportResult(&ScheduledTestResult{PelicanConfig: p.PelicanConfig, Status: "failed"})
		if k == "" || kinds[k] {
			return ""
		}
		kinds[k] = true
		members = append(members, p)
		own = own || p.ID == plan.ID
	}
	if !own || !kinds["candy"] || !kinds["pelican"] || members[0].ID == members[1].ID {
		return ""
	}
	sort.Slice(members, func(i, j int) bool { return members[i].ID < members[j].ID })
	identity := []any{plan.AccountID, plan.PelicanConfig.ReportPairKey, plan.ModelID, plan.CronExpression, zone}
	for _, member := range members {
		identity = append(identity, member.ID, member.UpdatedAt.UTC(), member.PelicanConfig)
	}
	raw, _ := json.Marshal(identity)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Pair identity uses explicit peer plans and their original due time; never near-time pairing.
func PelicanReportPairIdentity(plan *ScheduledTestPlan, peers []*ScheduledTestPlan, zone string) string {
	fingerprint := PelicanReportPairFingerprint(plan, peers, zone)
	if fingerprint == "" || plan.NextRunAt == nil || plan.NextRunAt.IsZero() {
		return ""
	}
	for _, p := range peers {
		if p.NextRunAt == nil || !p.NextRunAt.Equal(*plan.NextRunAt) {
			return ""
		}
	}
	sum := sha256.Sum256([]byte(fingerprint + "\x00" + plan.NextRunAt.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:])
}
func reportString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func reportDuration(start, end *time.Time) *int64 {
	if start == nil || end == nil || start.IsZero() || end.IsZero() || end.Before(*start) {
		return nil
	}
	n := end.Sub(*start).Milliseconds()
	return &n
}
func publicReportResult(f PelicanReportFact) PelicanReportResult {
	if f.StartedAt != nil && f.StartedAt.IsZero() {
		f.StartedAt = nil
	}
	if f.CompletedAt != nil && f.CompletedAt.IsZero() {
		f.CompletedAt = nil
	}
	if f.StartedAt != nil && f.CompletedAt != nil && f.CompletedAt.Before(*f.StartedAt) {
		f.StartedAt, f.CompletedAt = nil, nil
	}
	return PelicanReportResult{ResultID: f.ResultID, ExecutionID: reportString(f.ExecutionID), SharedRoundID: reportString(f.SharedRoundID), ScheduledFor: f.ScheduledFor, Status: f.Status, Judgment: f.Judgment, StartedAt: f.StartedAt, CompletedAt: f.CompletedAt, LatencyMs: reportDuration(f.StartedAt, f.CompletedAt)}
}
func reportStatistics(rows []PelicanReportFact) PelicanReportStats {
	s := PelicanReportStats{}
	ids := map[string]bool{}
	legacy := false
	var duration int64
	for _, f := range rows {
		s.ObservedCount++
		if f.ExecutionID == "" {
			legacy = true
		} else {
			ids[f.ExecutionID] = true
		}
		if f.Status == "ungraded" {
			s.UngradedCount++
			continue
		}
		s.TotalCount++
		if f.Status == "success" {
			s.SuccessCount++
		} else {
			s.FailureCount++
		}
		if ms := reportDuration(f.StartedAt, f.CompletedAt); ms != nil {
			s.TimedCount++
			duration += *ms
		}
	}
	if !legacy {
		n := int64(len(ids))
		s.ExecutionCount = &n
	}
	if s.TotalCount > 0 {
		n := float64(s.SuccessCount) * 100 / float64(s.TotalCount)
		s.SuccessRate = &n
	}
	if s.TimedCount > 0 {
		n := float64(duration) / float64(s.TimedCount)
		s.AvgLatencyMs = &n
	}
	return s
}
func latestReportExecution(rows []PelicanReportFact) *PelicanReportExecution {
	if len(rows) == 0 {
		return nil
	}
	last := rows[len(rows)-1]
	out := &PelicanReportExecution{ExecutionID: reportString(last.ExecutionID), SharedRoundID: reportString(last.SharedRoundID), ScheduledFor: last.ScheduledFor, Status: "success", Results: []PelicanReportResult{}}
	if last.ExecutionID != "" {
		n := last.ExpectedCount
		out.ExpectedCount = &n
	}
	timesValid := true
	for _, f := range rows {
		if (last.ExecutionID != "" && f.ExecutionID != last.ExecutionID) || (last.ExecutionID == "" && f.ResultID != last.ResultID) {
			continue
		}
		out.Results = append(out.Results, publicReportResult(f))
		out.CompletedCount++
		if f.Status == "failed" {
			out.Status = "failed"
		} else if f.Status == "ungraded" && out.Status != "failed" {
			out.Status = "ungraded"
		}
		if f.ExpectedCount != last.ExpectedCount || f.SharedRoundID != last.SharedRoundID {
			out.SharedRoundID = nil
			timesValid = false
		}
		if reportDuration(f.StartedAt, f.CompletedAt) == nil {
			timesValid = false
			continue
		}
		if out.StartedAt == nil || f.StartedAt.Before(*out.StartedAt) {
			out.StartedAt = f.StartedAt
		}
		if out.CompletedAt == nil || f.CompletedAt.After(*out.CompletedAt) {
			out.CompletedAt = f.CompletedAt
		}
	}
	if out.ExpectedCount != nil && (out.CompletedCount != *out.ExpectedCount || *out.ExpectedCount < 1) {
		timesValid = false
		out.Status = "pending"
	}
	if !timesValid {
		out.StartedAt = nil
		out.CompletedAt = nil
	} else {
		out.LatencyMs = reportDuration(out.StartedAt, out.CompletedAt)
	}
	return out
}
func pairedReportRound(a, b *PelicanReportExecution) *PelicanReportRound {
	if a == nil || b == nil || a.ExecutionID == nil || b.ExecutionID == nil || a.SharedRoundID == nil || b.SharedRoundID == nil || *a.SharedRoundID != *b.SharedRoundID || a.ScheduledFor == nil || b.ScheduledFor == nil || !a.ScheduledFor.Equal(*b.ScheduledFor) || a.LatencyMs == nil || b.LatencyMs == nil {
		return nil
	}
	start, end := *a.StartedAt, *a.CompletedAt
	if b.StartedAt.Before(start) {
		start = *b.StartedAt
	}
	if b.CompletedAt.After(end) {
		end = *b.CompletedAt
	}
	return &PelicanReportRound{RoundID: *a.SharedRoundID, ScheduledFor: *a.ScheduledFor, StartedAt: start, CompletedAt: end, LatencyMs: end.Sub(start).Milliseconds(), CandyExecutionID: *a.ExecutionID, PelicanExecutionID: *b.ExecutionID}
}
func recentReportFacts(facts []PelicanReportFact) []PelicanReportFact {
	if len(facts) > PelicanReportHistoryLimit {
		return facts[len(facts)-PelicanReportHistoryLimit:]
	}
	return facts
}

func buildPelicanReport(data *PelicanReportData, now time.Time) *PelicanReportView {
	v := &PelicanReportView{SchemaVersion: 2, AsOf: now, Group: data.Group, ModelID: reportString(data.ModelID)}
	v.Window.From = now.Add(-24 * time.Hour)
	v.Window.To = now
	all := map[string][]PelicanReportFact{"candy": {}, "pelican": {}}
	window := map[string][]PelicanReportFact{"candy": {}, "pelican": {}}
	sort.Slice(data.Facts, func(i, j int) bool {
		a, b := data.Facts[i], data.Facts[j]
		if a.ObservedAt.Equal(b.ObservedAt) {
			return a.ResultID < b.ResultID
		}
		return a.ObservedAt.Before(b.ObservedAt)
	})
	for _, f := range data.Facts {
		if f.ModelID != data.ModelID || !f.ObservedAt.Before(now) || (f.Kind != "candy" && f.Kind != "pelican") {
			continue
		}
		all[f.Kind] = append(all[f.Kind], f)
		if !f.ObservedAt.Before(v.Window.From) {
			window[f.Kind] = append(window[f.Kind], f)
		}
	}
	v.History.Candy = []PelicanReportResult{}
	v.History.Pelican = []PelicanReportResult{}
	for _, f := range recentReportFacts(window["candy"]) {
		v.History.Candy = append(v.History.Candy, publicReportResult(f))
	}
	for _, f := range recentReportFacts(window["pelican"]) {
		v.History.Pelican = append(v.History.Pelican, publicReportResult(f))
	}
	v.Statistics.Candy = reportStatistics(window["candy"])
	v.Statistics.Pelican = reportStatistics(window["pelican"])
	if data.Statistics != nil {
		v.Statistics = *data.Statistics
	}
	v.HistoryMeta.Candy = PelicanReportHistoryMeta{TotalCount: v.Statistics.Candy.ObservedCount, ReturnedCount: len(v.History.Candy), Limit: PelicanReportHistoryLimit}
	v.HistoryMeta.Pelican = PelicanReportHistoryMeta{TotalCount: v.Statistics.Pelican.ObservedCount, ReturnedCount: len(v.History.Pelican), Limit: PelicanReportHistoryLimit}
	v.Latest.Candy = latestReportExecution(all["candy"])
	v.Latest.Pelican = latestReportExecution(all["pelican"])
	v.CurrentRound = pairedReportRound(v.Latest.Candy, v.Latest.Pelican)
	if p := v.Latest.Pelican; p != nil && p.Status == "success" && (p.ExpectedCount == nil || p.CompletedCount == *p.ExpectedCount) && data.Artwork != nil && data.Artwork.GroupID == data.Group.ID && data.Artwork.ModelID == data.ModelID {
		for _, r := range p.Results {
			if r.ResultID == data.Artwork.SourceResultID {
				v.Artwork = data.Artwork
				v.Artwork.ExecutionID = p.ExecutionID
				break
			}
		}
	}
	identity := *v
	identity.AsOf = time.Time{}
	identity.Window.From = time.Time{}
	identity.Window.To = time.Time{}
	raw, _ := json.Marshal(identity)
	sum := sha256.Sum256(raw)
	v.SnapshotID = "sha256:" + hex.EncodeToString(sum[:])
	return v
}
