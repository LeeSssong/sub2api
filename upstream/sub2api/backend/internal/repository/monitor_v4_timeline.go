package repository

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"time"
)

// Shared with the aggregate projection: identical terminal deduplication and exclusions.
const monitorV4RealEventsSQL = `WITH scopes AS (
  SELECT group_id, account_id
  FROM unnest($4::bigint[], $5::bigint[]) AS scope(group_id, account_id)
), groups AS (
  SELECT unnest($6::bigint[]) AS group_id
), bucket_bounds AS (
  SELECT date_bin($3::interval, $1::timestamptz, TIMESTAMPTZ '2001-01-01 00:00:00+00') AS start_bucket,
         date_bin($3::interval, $2::timestamptz, TIMESTAMPTZ '2001-01-01 00:00:00+00') AS end_bucket
), buckets AS (
  SELECT series.bucket_start
  FROM bucket_bounds bb
  CROSS JOIN LATERAL generate_series(
         CASE WHEN bb.start_bucket < $1::timestamptz THEN bb.start_bucket + $3::interval ELSE bb.start_bucket END,
         bb.end_bucket - $3::interval,
         $3::interval
       ) AS series(bucket_start)
  WHERE series.bucket_start + $3::interval <= $2::timestamptz
  UNION ALL
  SELECT date_bin($3::interval, $2::timestamptz, TIMESTAMPTZ '2001-01-01 00:00:00+00')
  WHERE $2::timestamptz >= date_bin($3::interval, $2::timestamptz, TIMESTAMPTZ '2001-01-01 00:00:00+00') + $3::interval - interval '1 minute'
    AND $2::timestamptz < date_bin($3::interval, $2::timestamptz, TIMESTAMPTZ '2001-01-01 00:00:00+00') + $3::interval
), raw_usage_candidates AS (
  SELECT u.group_id, u.account_id, u.id::bigint AS source_id, u.created_at AS observed_at,
	         date_bin($3::interval, u.created_at, TIMESTAMPTZ '2001-01-01 00:00:00+00') AS bucket_start,
         (COALESCE(NULLIF(u.usage_completeness, ''), 'complete') = 'complete') AS successful,
         u.first_token_ms::double precision AS first_token_ms,
         u.duration_ms::double precision AS duration_ms,
	         COALESCE(u.input_tokens, 0)::double precision AS input_tokens,
	         COALESCE(u.cache_creation_tokens, 0)::double precision AS cache_creation_tokens,
         COALESCE(u.cache_read_tokens, 0)::double precision AS cache_read_tokens,
         COALESCE(NULLIF(u.logical_request_id, ''), NULLIF(u.request_id, ''), 'usage:' || u.id::text) AS request_key,
         NULLIF(u.logical_request_id, '') AS logical_request_id,
         NULLIF(u.request_id, '') AS request_id,
         1 AS source_priority
  FROM usage_logs u
  JOIN groups g ON g.group_id = u.group_id
  WHERE u.created_at >= $1::timestamptz AND u.created_at < $2::timestamptz
    AND NOT (u.created_at >= TIMESTAMPTZ '2026-08-31 00:00:00+08' AND u.created_at < TIMESTAMPTZ '2026-09-02 00:00:00+08')
    AND u.usage_completeness IS DISTINCT FROM 'unknown'
), cache_usage AS (
  SELECT group_id,
         COALESCE(SUM(input_tokens), 0)::bigint AS input_tokens,
         COALESCE(SUM(cache_read_tokens), 0)::bigint AS cache_read_tokens,
         COALESCE(SUM(cache_creation_tokens), 0)::bigint AS cache_creation_tokens,
         COALESCE(SUM(input_tokens + cache_creation_tokens + cache_read_tokens), 0)::bigint AS cache_hit_denominator,
         SUM(cache_read_tokens)
           / NULLIF(SUM(input_tokens + cache_creation_tokens + cache_read_tokens), 0) AS cache_hit_rate
  FROM raw_usage_candidates
  WHERE successful
  GROUP BY group_id
), unknown_usage_keys AS (
  SELECT DISTINCT u.group_id, NULLIF(u.request_id, '') AS request_key
  FROM usage_logs u
  JOIN groups g ON g.group_id = u.group_id
  WHERE u.created_at >= $1::timestamptz AND u.created_at < $2::timestamptz
    AND u.usage_completeness = 'unknown'
    AND NULLIF(u.request_id, '') IS NOT NULL
  UNION
  SELECT DISTINCT u.group_id, NULLIF(u.logical_request_id, '') AS request_key
  FROM usage_logs u
  JOIN groups g ON g.group_id = u.group_id
  WHERE u.created_at >= $1::timestamptz AND u.created_at < $2::timestamptz
    AND u.usage_completeness = 'unknown'
    AND NULLIF(u.logical_request_id, '') IS NOT NULL
), excluded_usage_keys AS (
  SELECT o.group_id, o.account_id, NULLIF(o.request_id, '') AS request_key
  FROM ops_error_logs o
  JOIN groups g ON g.group_id = o.group_id
  WHERE o.created_at >= $1::timestamptz AND o.created_at < $2::timestamptz
    AND NOT (o.created_at >= TIMESTAMPTZ '2026-08-31 00:00:00+08' AND o.created_at < TIMESTAMPTZ '2026-09-02 00:00:00+08')
    AND COALESCE(o.status_code, 0) >= 400
    AND COALESCE(o.is_count_tokens, FALSE) = FALSE
    AND NULLIF(o.request_id, '') IS NOT NULL
    AND (
      COALESCE(o.error_owner, '') = 'client'
      OR (COALESCE(o.error_phase, '') = 'request' AND COALESCE(o.error_source, '') = 'client_request')
      OR lower(CONCAT_WS(' ', o.error_type, o.error_message, o.error_body, o.upstream_error_message, o.upstream_error_detail)) LIKE ANY (ARRAY[
        '%model not supported%', '%unsupported model%', '%not supported by any configured account%',
        '%does not support the requested model%', '%model_not_supported%', '%model_unsupported%',
        '%model_not_found%', '%本站暂不支持%'
      ])
    )
  UNION
  SELECT o.group_id, o.account_id, NULLIF(o.client_request_id, '') AS request_key
  FROM ops_error_logs o
  JOIN groups g ON g.group_id = o.group_id
  WHERE o.created_at >= $1::timestamptz AND o.created_at < $2::timestamptz
    AND NOT (o.created_at >= TIMESTAMPTZ '2026-08-31 00:00:00+08' AND o.created_at < TIMESTAMPTZ '2026-09-02 00:00:00+08')
    AND COALESCE(o.status_code, 0) >= 400
    AND COALESCE(o.is_count_tokens, FALSE) = FALSE
    AND NULLIF(o.client_request_id, '') IS NOT NULL
    AND (
      COALESCE(o.error_owner, '') = 'client'
      OR (COALESCE(o.error_phase, '') = 'request' AND COALESCE(o.error_source, '') = 'client_request')
      OR lower(CONCAT_WS(' ', o.error_type, o.error_message, o.error_body, o.upstream_error_message, o.upstream_error_detail)) LIKE ANY (ARRAY[
        '%model not supported%', '%unsupported model%', '%not supported by any configured account%',
        '%does not support the requested model%', '%model_not_supported%', '%model_unsupported%',
        '%model_not_found%', '%本站暂不支持%'
      ])
    )
), usage_candidates AS (
  SELECT u.*
  FROM raw_usage_candidates u
  LEFT JOIN excluded_usage_keys request_exclusion
    ON request_exclusion.group_id = u.group_id
   AND request_exclusion.account_id IS NOT DISTINCT FROM u.account_id
   AND request_exclusion.request_key = u.request_id
  LEFT JOIN excluded_usage_keys logical_exclusion
    ON logical_exclusion.group_id = u.group_id
   AND logical_exclusion.account_id IS NOT DISTINCT FROM u.account_id
   AND logical_exclusion.request_key = u.logical_request_id
  WHERE request_exclusion.request_key IS NULL
    AND logical_exclusion.request_key IS NULL
), usage_request_keys AS (
  -- request_id is the only exact bridge available on ops_error_logs. Keep
  -- the mapping group-scoped so failover across accounts remains one request.
  SELECT DISTINCT group_id, request_id AS request_key, request_key AS canonical_request_key
  FROM usage_candidates
  WHERE request_id IS NOT NULL
), error_candidates AS (
  SELECT o.group_id, o.account_id, o.id::bigint AS source_id, o.created_at AS observed_at,
         date_bin($3::interval, o.created_at, TIMESTAMPTZ '2001-01-01 00:00:00+00') AS bucket_start,
         FALSE AS successful,
         NULL::double precision AS first_token_ms,
         NULL::double precision AS duration_ms,
	         NULL::double precision AS input_tokens,
	         NULL::double precision AS cache_creation_tokens,
         NULL::double precision AS cache_read_tokens,
         COALESCE(request_match.canonical_request_key, NULLIF(o.request_id, ''), 'error:' || o.id::text) AS request_key,
         0 AS source_priority
  FROM ops_error_logs o
  JOIN groups g ON g.group_id = o.group_id
  LEFT JOIN usage_request_keys request_match
    ON request_match.group_id = o.group_id
   AND request_match.request_key = NULLIF(o.request_id, '')
  WHERE o.created_at >= $1::timestamptz AND o.created_at < $2::timestamptz
    AND NOT (o.created_at >= TIMESTAMPTZ '2026-08-31 00:00:00+08' AND o.created_at < TIMESTAMPTZ '2026-09-02 00:00:00+08')
    AND COALESCE(o.is_count_tokens, FALSE) = FALSE
    AND COALESCE(o.status_code, 0) >= 400
    AND NOT (
      COALESCE(o.error_owner, '') = 'client'
      OR (COALESCE(o.error_phase, '') = 'request' AND COALESCE(o.error_source, '') = 'client_request')
      OR lower(CONCAT_WS(' ', o.error_type, o.error_message, o.error_body, o.upstream_error_message, o.upstream_error_detail)) LIKE ANY (ARRAY[
        '%model not supported%', '%unsupported model%', '%not supported by any configured account%',
        '%does not support the requested model%', '%model_not_supported%', '%model_unsupported%',
        '%model_not_found%', '%本站暂不支持%'
      ])
    )
    AND NOT EXISTS (
      SELECT 1
      FROM unknown_usage_keys unknown_usage
      WHERE unknown_usage.group_id = o.group_id
        AND unknown_usage.request_key IN (NULLIF(o.request_id, ''), NULLIF(o.client_request_id, ''))
    )
), real_candidates AS (
  SELECT group_id, account_id, source_id, observed_at, bucket_start, successful, first_token_ms, duration_ms, input_tokens, cache_creation_tokens, cache_read_tokens, request_key, source_priority
  FROM usage_candidates
  UNION ALL
  SELECT group_id, account_id, source_id, observed_at, bucket_start, successful, first_token_ms, duration_ms, input_tokens, cache_creation_tokens, cache_read_tokens, request_key, source_priority
  FROM error_candidates
), real_events AS (
  SELECT group_id, account_id, observed_at, bucket_start, successful, first_token_ms, duration_ms, input_tokens, cache_creation_tokens, cache_read_tokens, request_key, 'real'::text AS source
  FROM (
    SELECT rc.*, ROW_NUMBER() OVER (
      PARTITION BY rc.group_id, rc.request_key
      ORDER BY rc.observed_at DESC, rc.successful DESC, rc.source_id DESC
    ) AS position
    FROM real_candidates rc
  ) ranked
  WHERE position = 1
)`

func (r *accountMonitorRepository) ReadMonitorV4Timeline(ctx context.Context, ids []int64, start, end time.Time, step time.Duration) ([]service.MonitorV4TimelinePoint, error) {
	out := []service.MonitorV4TimelinePoint{}
	if len(ids) == 0 {
		return out, nil
	}
	if len(ids) > 100 || step < 5*time.Minute || !end.After(start) || end.Sub(start) > 7*24*time.Hour || end.Sub(start)/step > 168 {
		return nil, fmt.Errorf("invalid timeline bounds")
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("invalid timeline group")
		}
	}
	rows, err := r.db.QueryContext(ctx, monitorV4RealEventsSQL+`
 SELECT g.group_id, series.bucket_start,
 COUNT(e.observed_at) AS request_count, COUNT(e.observed_at) FILTER (WHERE e.successful) AS success_count,
 SUM(e.cache_read_tokens) FILTER (WHERE e.successful)
 / NULLIF(SUM(e.input_tokens + e.cache_creation_tokens + e.cache_read_tokens) FILTER (WHERE e.successful), 0) AS cache_hit_rate,
 PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY e.first_token_ms)
 FILTER (WHERE e.successful AND e.first_token_ms IS NOT NULL) AS ttft_p50_ms
 FROM groups g CROSS JOIN LATERAL generate_series($1::timestamptz,$2::timestamptz-$3::interval,$3::interval) AS series(bucket_start)
 LEFT JOIN real_events e ON e.group_id=g.group_id AND e.observed_at>=series.bucket_start AND e.observed_at<series.bucket_start+$3::interval
 GROUP BY g.group_id,series.bucket_start ORDER BY g.group_id,series.bucket_start
 `, start.UTC(), end.UTC(), step.String(), pq.Array([]int64{}), pq.Array([]int64{}), pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p service.MonitorV4TimelinePoint
		if err := rows.Scan(&p.GroupID, &p.Start, &p.RequestCount, &p.SuccessCount, &p.CacheHitRate, &p.TTFTP50MS); err != nil {
			return nil, err
		}
		p.End = p.Start.Add(step)
		out = append(out, p)
	}
	return out, rows.Err()
}
