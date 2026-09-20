ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS turn_state_inject_enabled boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN groups.turn_state_inject_enabled IS
    'Whether accounts in this OpenAI group participate in reusable Codex turn-state injection';
