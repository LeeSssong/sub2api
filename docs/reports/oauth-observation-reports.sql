\set ON_ERROR_STOP on
\pset pager off
\if :{?from}
\else
SELECT (now()-interval '1 day')::text AS "from" \gset
\endif
\if :{?to}
\else
SELECT now()::text AS "to" \gset
\endif
\if :{?account_id}
\else
\set account_id 0
\endif
BEGIN READ ONLY;
SET LOCAL statement_timeout='30s';
SET LOCAL TIME ZONE 'Asia/Shanghai';
SELECT :'to'::timestamptz>:'from'::timestamptz AND :'to'::timestamptz-:'from'::timestamptz<=interval '7 days' AS valid_window \gset
\if :valid_window
\else
\echo 'Report window must be positive and at most seven days.'
\quit 2
\endif

\echo 'Account lifetime: only probe healthy -> later probe degraded; never operational state.'
SELECT e.account_id, a.name AS current_account_name, e.subject_id, e.identity_conflict,
 e.first_observed_at AS observation_started_at,e.last_probe_at,
 l.first_healthy_at,l.first_degraded_at,
 CASE WHEN e.identity_conflict THEN 'identity_changed_review_required'
      WHEN l.first_healthy_at IS NULL THEN 'not_started'
      WHEN l.first_degraded_at IS NULL THEN 'no_degraded_observed'
      ELSE 'ended' END AS lifetime_state,
 extract(epoch FROM(l.first_degraded_at-l.first_healthy_at))/60 AS lifetime_minutes,
 extract(epoch FROM(COALESCE(l.first_degraded_at,e.last_probe_at)-l.first_healthy_at))/60 AS observed_span_minutes
FROM oauth_observation_episodes e
LEFT JOIN oauth_observation_episode_lifetimes l ON l.episode_account_id=e.account_id
LEFT JOIN accounts a ON a.id=e.account_id
WHERE (:'account_id'::bigint=0 OR e.account_id=:'account_id'::bigint)
 AND e.first_observed_at<:'to'::timestamptz
 AND (e.first_observed_at>=:'from'::timestamptz OR e.last_probe_at>=:'from'::timestamptz)
ORDER BY e.account_id;

\echo 'Cohorts: ended-only median is descriptive, not survival-adjusted.'
SELECT date_trunc('day',l.first_healthy_at) AS cohort_day,
 count(*) episodes,count(l.first_degraded_at) ended,
 count(*) FILTER(WHERE l.first_degraded_at IS NULL) no_degraded_observed,
 percentile_cont(.5) WITHIN GROUP(ORDER BY extract(epoch FROM(l.first_degraded_at-l.first_healthy_at))/60)
 FILTER(WHERE l.first_degraded_at IS NOT NULL) AS ended_only_median_minutes
FROM oauth_observation_episode_lifetimes l JOIN oauth_observation_episodes e ON e.account_id=l.episode_account_id
WHERE NOT e.identity_conflict AND l.first_healthy_at>=:'from'::timestamptz AND l.first_healthy_at<:'to'::timestamptz
 AND (:'account_id'::bigint=0 OR e.account_id=:'account_id'::bigint)
GROUP BY 1 ORDER BY 1;

\echo 'Probe/selection/outcome timeline; generated attempt_id correlates attempts without client identifiers.'
SELECT occurred_at,episode_account_id,event_type,payload,
 extract(epoch FROM(recorded_at-occurred_at)) AS ingest_delay_seconds
FROM oauth_observation_events
WHERE occurred_at>=:'from'::timestamptz AND occurred_at<:'to'::timestamptz
 AND (:'account_id'::bigint=0 OR episode_account_id=:'account_id'::bigint)
ORDER BY occurred_at,id LIMIT 5000;

\echo 'Usage minute facts copied from native usage_logs; total_cost is standard USD, actual_cost is site charge.'
SELECT minute_at,account_id,model,protocol,requests,total_cost,actual_cost,
 input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,
 complete_requests,partial_requests,unknown_requests,
 latency_ms_total::numeric/NULLIF(latency_count,0) AS average_duration_ms,
 first_token_ms_total::numeric/NULLIF(first_token_count,0) AS average_first_token_ms
FROM oauth_observation_usage_minutes
WHERE minute_at>=:'from'::timestamptz AND minute_at<:'to'::timestamptz AND requests>0
 AND (:'account_id'::bigint=0 OR account_id=:'account_id'::bigint)
ORDER BY minute_at,account_id,model,protocol;

\echo 'Exposure during probe-defined lifetime, with reasoning/tier and request-level latency percentiles.'
SELECT u.account_id,u.model,u.protocol,u.reasoning_effort,u.service_tier,u.long_context_billing_applied,
 count(*) requests,sum(u.total_cost) standard_usd,sum(u.input_tokens) input_tokens,
 sum(u.output_tokens) output_tokens,sum(u.cache_read_tokens) cached_tokens,
 percentile_cont(.5) WITHIN GROUP(ORDER BY u.duration_ms) AS duration_p50_ms,
 percentile_cont(.95) WITHIN GROUP(ORDER BY u.duration_ms) AS duration_p95_ms
FROM oauth_observation_usage_contributions u
JOIN oauth_observation_episode_lifetimes l ON l.episode_account_id=u.account_id
WHERE u.occurred_at>=GREATEST(l.first_healthy_at,:'from'::timestamptz)
 AND u.occurred_at<LEAST(COALESCE(l.first_degraded_at,:'to'::timestamptz),:'to'::timestamptz)
 AND (:'account_id'::bigint=0 OR u.account_id=:'account_id'::bigint)
GROUP BY 1,2,3,4,5,6 ORDER BY 1,standard_usd DESC;

\echo 'Minute occupancy from slot boundaries across workers. Open/lost/expired observations make average/peak NULL.'
SELECT * FROM oauth_observation_occupancy(:'from'::timestamptz,:'to'::timestamptz,NULLIF(:'account_id'::bigint,0));

\echo 'Operational and quality-rule history (not lifetime endpoints); no raw credential/content fields.'
SELECT observed_at,subject_type,subject_key,operation,snapshot
FROM oauth_observation_archives
WHERE observed_at>=:'from'::timestamptz AND observed_at<:'to'::timestamptz
 AND (:'account_id'::bigint=0 OR snapshot->>'account_id'=:'account_id'
      OR (subject_type IN('account','admin_account_mutation') AND subject_key=:'account_id'))
ORDER BY observed_at,id LIMIT 5000;

\echo 'Runtime result/filter counts by minute. These count attempts/candidate decisions, not billed requests.'
SELECT date_trunc('minute',occurred_at) AS minute_at,episode_account_id,event_type,
 payload->>'model' model,payload->>'error_class' reason,count(*) observations
FROM oauth_observation_events
WHERE occurred_at>=:'from'::timestamptz AND occurred_at<:'to'::timestamptz
 AND event_type IN('request_outcome','live_call_outcome','candidate_filtered','slot_denied','fallback')
 AND (:'account_id'::bigint=0 OR episode_account_id=:'account_id'::bigint)
GROUP BY 1,2,3,4,5 ORDER BY 1,2;

\echo 'Recorder coverage history: counters are process-cumulative, include prefilter submissions, and are not billable counts.'
SELECT instance_id,minute_at,observed_at,enqueued,persisted,dropped,write_errors,queue_depth
FROM oauth_observation_recorder_health_minutes
WHERE minute_at>=:'from'::timestamptz AND minute_at<:'to'::timestamptz
ORDER BY minute_at,instance_id;
COMMIT;
