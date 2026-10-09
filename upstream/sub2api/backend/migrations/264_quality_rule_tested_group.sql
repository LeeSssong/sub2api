-- Preserve the tested route when a group rule is edited later. NULL means the
-- rule covers all/ungrouped accounts, or its historical route cannot be proved.
SET LOCAL lock_timeout = '100ms';
SET LOCAL statement_timeout = '2s';

ALTER TABLE quality_rule_template_accounts
    ADD COLUMN IF NOT EXISTS tested_group_id BIGINT;

-- Existing links are only attributable if the template has not been edited
-- since they were created. Do not relabel ambiguous history using today's filter.
UPDATE quality_rule_template_accounts link
SET tested_group_id = CASE
    WHEN template.account_filter->>'group' ~ '^[1-9][0-9]{0,18}$' THEN
        CASE WHEN (template.account_filter->>'group')::numeric <= 9223372036854775807
             THEN (template.account_filter->>'group')::bigint END
    END
FROM quality_rule_templates template
WHERE link.template_id = template.id
  AND link.tested_group_id IS NULL
  AND link.created_at >= template.updated_at;
