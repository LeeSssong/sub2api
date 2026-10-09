package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func recordQualityTrafficVerdict(ctx context.Context, tx *sql.Tx, plan *service.ScheduledTestPlan) error {
	if plan.QualityTrafficVerdict != "healthy" && plan.QualityTrafficVerdict != "degraded" {
		return nil
	}
	// The immutable tested-group link survives account removal from the group.
	// Retain events independently of the seven-day scheduled-result cleanup.
	_, err := tx.ExecContext(ctx, `INSERT INTO account_quality_traffic_events(account_id,group_id,plan_id,degraded)
 SELECT $1,link.tested_group_id,$2,$3
 FROM quality_rule_template_accounts link
 WHERE link.plan_id=$2 AND link.tested_group_id IS NOT NULL
 AND (SELECT e.degraded FROM account_quality_traffic_events e
      WHERE e.account_id=$1 AND e.group_id=link.tested_group_id
      ORDER BY e.observed_at DESC,e.id DESC LIMIT 1) IS DISTINCT FROM $3::boolean`,
		plan.AccountID, plan.ID, plan.QualityTrafficVerdict == "degraded")
	return err
}
