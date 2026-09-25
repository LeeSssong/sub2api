-- Small independent outcome records survive raw-result, plan and artwork deletion.
-- No account, plan, prompt, model output, or credentials are stored here.
SET LOCAL lock_timeout = '100ms';
SET LOCAL statement_timeout = '2s';

CREATE TABLE IF NOT EXISTS pelican_drawing_outcomes (
    source_result_id BIGINT PRIMARY KEY,
    completed_at TIMESTAMPTZ NOT NULL,
    success BOOLEAN NOT NULL,
    group_ids BIGINT[] NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_pelican_drawing_outcomes_completed
    ON pelican_drawing_outcomes (completed_at, source_result_id);

-- Deliberately empty until the new collector starts. Migration time is not coverage.
CREATE TABLE IF NOT EXISTS pelican_drawing_statistics_state (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    coverage_started_at TIMESTAMPTZ NOT NULL
);

COMMENT ON TABLE pelican_drawing_outcomes IS
    'Terminal scheduled drawing outcomes with execution-start group attribution; independent 48-hour retention';
COMMENT ON COLUMN pelican_drawing_outcomes.source_result_id IS
    'Deduplication key only, intentionally no foreign key to pruned scheduled_test_results';
