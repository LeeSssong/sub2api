-- Permanently remove the retired custom OpenAI scheduler control plane.
-- Native scheduler switches, Top-K, and weight settings are intentionally kept.

DELETE FROM settings
WHERE key IN (
    'openai_advanced_scheduler_candidate_pool_mode',
    'openai_advanced_scheduler_exploration_ratio',
    'openai_advanced_scheduler_starvation_threshold_seconds',
    'openai_advanced_scheduler_fairness_weight',
    'openai_advanced_scheduler_group_overrides',
    'openai_advanced_scheduler_group_policies',
    'openai_advanced_scheduler_custom_presets'
);

DROP TABLE IF EXISTS openai_scheduler_logs;
