-- Additive, privacy-preserving observations for OpenAI OAuth account episodes.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS oauth_observation_identity_secrets (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    secret BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO oauth_observation_identity_secrets (id, secret)
VALUES (TRUE, gen_random_bytes(32)) ON CONFLICT (id) DO NOTHING;
REVOKE ALL ON TABLE oauth_observation_identity_secrets FROM PUBLIC;

CREATE TABLE IF NOT EXISTS oauth_observation_subjects (
    id BIGSERIAL PRIMARY KEY,
    identity_hmac BYTEA NOT NULL UNIQUE,
    first_observed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE IF NOT EXISTS oauth_observation_episodes (
    account_id BIGINT PRIMARY KEY,
    subject_id BIGINT,
    first_observed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_observed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    last_probe_at TIMESTAMPTZ,
    identity_conflict BOOLEAN NOT NULL DEFAULT FALSE,
    -- No foreign keys: account and subject deletion must not erase observations.
    CHECK (subject_id IS NULL OR subject_id > 0)
);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_episodes_subject ON oauth_observation_episodes(subject_id);

CREATE TABLE IF NOT EXISTS oauth_observation_events (
    id BIGSERIAL PRIMARY KEY,
    event_key TEXT NOT NULL UNIQUE,
    account_id BIGINT NOT NULL,
    episode_account_id BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_events_episode_time ON oauth_observation_events(episode_account_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_events_type_time ON oauth_observation_events(event_type, occurred_at);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_events_recorded_at ON oauth_observation_events(recorded_at);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_probe_events_episode_time ON oauth_observation_events(episode_account_id, occurred_at) WHERE event_type='probe_result';

CREATE TABLE IF NOT EXISTS oauth_observation_probe_lifetimes (
    episode_account_id BIGINT NOT NULL,
    model TEXT NOT NULL,
    protocol TEXT NOT NULL,
    probe_version TEXT NOT NULL,
    first_healthy_at TIMESTAMPTZ NOT NULL,
    first_degraded_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (episode_account_id, model, protocol, probe_version)
);
CREATE TABLE IF NOT EXISTS oauth_observation_episode_lifetimes (
    episode_account_id BIGINT PRIMARY KEY,
    first_healthy_at TIMESTAMPTZ NOT NULL,
    first_degraded_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE IF NOT EXISTS oauth_observation_slots (
    id BIGSERIAL PRIMARY KEY,
    event_key TEXT NOT NULL UNIQUE,
    account_id BIGINT NOT NULL,
    episode_account_id BIGINT NOT NULL,
    slot_id TEXT NOT NULL,
    attempt_id TEXT,
    slot_role TEXT,
    model TEXT,
    acquired_at TIMESTAMPTZ NOT NULL,
    released_at TIMESTAMPTZ,
    release_state TEXT NOT NULL DEFAULT 'open' CHECK (release_state IN ('open', 'released', 'release_failed')),
    protocol TEXT NOT NULL DEFAULT 'unknown',
    slot_limit INTEGER,
    expires_at TIMESTAMPTZ,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_oauth_observation_slots_episode_slot ON oauth_observation_slots(episode_account_id, slot_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_observation_slots_identity ON oauth_observation_slots(episode_account_id,slot_id);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_slots_episode_acquired ON oauth_observation_slots(episode_account_id, acquired_at);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_slots_open ON oauth_observation_slots(acquired_at) WHERE released_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_oauth_observation_slots_recorded_at ON oauth_observation_slots(recorded_at);

CREATE TABLE IF NOT EXISTS oauth_observation_usage_contributions (
    usage_log_id BIGINT PRIMARY KEY,
    account_id BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    minute_at TIMESTAMPTZ NOT NULL,
    model TEXT NOT NULL,
    protocol TEXT NOT NULL,
    input_tokens BIGINT NOT NULL,
    output_tokens BIGINT NOT NULL,
    cache_creation_tokens BIGINT NOT NULL,
    cache_read_tokens BIGINT NOT NULL,
    actual_cost NUMERIC(20,10) NOT NULL,
    total_cost NUMERIC(20,10) NOT NULL,
    duration_ms BIGINT,
    first_token_ms BIGINT,
    reasoning_effort TEXT,
    service_tier TEXT,
    long_context_billing_applied BOOLEAN NOT NULL DEFAULT FALSE,
    usage_completeness TEXT NOT NULL DEFAULT 'unknown'
);
CREATE TABLE IF NOT EXISTS oauth_observation_usage_minutes (
    account_id BIGINT NOT NULL,
    minute_at TIMESTAMPTZ NOT NULL,
    model TEXT NOT NULL,
    protocol TEXT NOT NULL,
    requests BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    actual_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    total_cost NUMERIC(20,10) NOT NULL DEFAULT 0,
    latency_count BIGINT NOT NULL DEFAULT 0,
    latency_ms_total BIGINT NOT NULL DEFAULT 0,
    first_token_count BIGINT NOT NULL DEFAULT 0,
    first_token_ms_total BIGINT NOT NULL DEFAULT 0,
    complete_requests BIGINT NOT NULL DEFAULT 0,
    partial_requests BIGINT NOT NULL DEFAULT 0,
    unknown_requests BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, minute_at, model, protocol)
);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_usage_minutes_time ON oauth_observation_usage_minutes(minute_at);

CREATE TABLE IF NOT EXISTS oauth_observation_archives (
    id BIGSERIAL PRIMARY KEY,
    subject_type TEXT NOT NULL,
    subject_key TEXT NOT NULL,
    operation TEXT NOT NULL CHECK (operation IN ('insert', 'update', 'delete')),
    observed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    snapshot JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_archives_subject_time ON oauth_observation_archives(subject_type, subject_key, observed_at);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_archives_observed_at ON oauth_observation_archives(observed_at);

CREATE TABLE IF NOT EXISTS oauth_observation_recorder_health (
    instance_id TEXT PRIMARY KEY,
    observed_at TIMESTAMPTZ NOT NULL,
    enqueued BIGINT NOT NULL,
    persisted BIGINT NOT NULL,
    dropped BIGINT NOT NULL,
    write_errors BIGINT NOT NULL,
    queue_depth INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS oauth_observation_recorder_health_minutes (
    instance_id TEXT NOT NULL, minute_at TIMESTAMPTZ NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL, enqueued BIGINT NOT NULL, persisted BIGINT NOT NULL,
    dropped BIGINT NOT NULL, write_errors BIGINT NOT NULL, queue_depth INTEGER NOT NULL,
    PRIMARY KEY(instance_id,minute_at)
);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_health_minutes_time ON oauth_observation_recorder_health_minutes(minute_at);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_contributions_time ON oauth_observation_usage_contributions(minute_at);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_contributions_account_time ON oauth_observation_usage_contributions(account_id,occurred_at);

CREATE OR REPLACE FUNCTION oauth_observation_account_is_eligible(p_account_id BIGINT)
RETURNS BOOLEAN LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
      SELECT 1 FROM accounts a
      WHERE a.id = p_account_id AND lower(a.platform) = 'openai' AND lower(a.type) = 'oauth'
    )
$$;

CREATE OR REPLACE FUNCTION oauth_observation_ensure_episode(p_account_id BIGINT, p_refresh_identity BOOLEAN DEFAULT FALSE)
RETURNS BIGINT LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE
    identity_value BYTEA;
    subject BIGINT;
    creds JSONB;
    new_episode BOOLEAN;
BEGIN
    IF EXISTS (SELECT 1 FROM accounts WHERE id=p_account_id AND (platform<>'openai' OR type<>'oauth')) THEN RETURN NULL; END IF;
    IF NOT p_refresh_identity AND EXISTS (SELECT 1 FROM oauth_observation_episodes WHERE account_id = p_account_id) THEN RETURN p_account_id; END IF;
    SELECT credentials INTO creds FROM accounts WHERE id = p_account_id;
    IF FOUND THEN
        IF NOT oauth_observation_account_is_eligible(p_account_id) THEN RETURN NULL; END IF;
        new_episode:=NOT EXISTS(SELECT 1 FROM oauth_observation_episodes WHERE account_id=p_account_id);
        IF NULLIF(creds->>'chatgpt_user_id', '') IS NOT NULL
           AND NULLIF(creds->>'chatgpt_account_id', '') IS NOT NULL THEN
            SELECT hmac(convert_to(length(creds->>'chatgpt_user_id')::TEXT || ':' || (creds->>'chatgpt_user_id') || length(creds->>'chatgpt_account_id')::TEXT || ':' || (creds->>'chatgpt_account_id'), 'UTF8'), secret, 'sha256')
              INTO identity_value FROM oauth_observation_identity_secrets WHERE id;
            INSERT INTO oauth_observation_subjects(identity_hmac) VALUES (identity_value)
              ON CONFLICT (identity_hmac) DO UPDATE SET identity_hmac = EXCLUDED.identity_hmac
              RETURNING id INTO subject;
        END IF;
        INSERT INTO oauth_observation_episodes(account_id, subject_id)
          VALUES (p_account_id, subject)
          ON CONFLICT (account_id) DO UPDATE SET subject_id = COALESCE(oauth_observation_episodes.subject_id, EXCLUDED.subject_id),
          identity_conflict=oauth_observation_episodes.identity_conflict OR (oauth_observation_episodes.subject_id IS NOT NULL AND EXCLUDED.subject_id IS NOT NULL AND oauth_observation_episodes.subject_id<>EXCLUDED.subject_id), last_observed_at = clock_timestamp();
        IF new_episode THEN
          INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot)
          SELECT 'account_baseline',p_account_id::text,'insert',jsonb_build_object('account_id',p_account_id,
            'source','observation_started','after',oauth_observation_account_snapshot(to_jsonb(a)))
          FROM accounts a WHERE a.id=p_account_id;
        END IF;
    END IF;
    RETURN (SELECT account_id FROM oauth_observation_episodes WHERE account_id = p_account_id);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_recompute_lifetime(p_episode BIGINT, p_model TEXT, p_protocol TEXT, p_version TEXT)
RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE healthy_at TIMESTAMPTZ; degraded_at TIMESTAMPTZ; old_healthy TIMESTAMPTZ; old_degraded TIMESTAMPTZ;
BEGIN
  -- Compact summaries are authoritative retained extrema after raw events are pruned.
  SELECT first_healthy_at, first_degraded_at INTO old_healthy, old_degraded
    FROM oauth_observation_episode_lifetimes WHERE episode_account_id=p_episode;
  SELECT min(x) INTO healthy_at FROM (SELECT old_healthy x UNION ALL SELECT occurred_at FROM oauth_observation_events WHERE episode_account_id=p_episode AND event_type='probe_result' AND payload->>'verdict'='healthy') q;
  IF healthy_at IS NOT NULL THEN
    SELECT min(x) INTO degraded_at FROM (SELECT old_degraded x WHERE old_degraded > healthy_at UNION ALL SELECT occurred_at FROM oauth_observation_events WHERE episode_account_id=p_episode AND event_type='probe_result' AND payload->>'verdict'='degraded' AND occurred_at > healthy_at) q;
    INSERT INTO oauth_observation_episode_lifetimes VALUES (p_episode,healthy_at,degraded_at,clock_timestamp())
    ON CONFLICT (episode_account_id) DO UPDATE SET first_healthy_at=EXCLUDED.first_healthy_at,first_degraded_at=EXCLUDED.first_degraded_at,updated_at=EXCLUDED.updated_at;
  END IF;
  SELECT first_healthy_at, first_degraded_at INTO old_healthy, old_degraded FROM oauth_observation_probe_lifetimes WHERE episode_account_id=p_episode AND model=p_model AND protocol=p_protocol AND probe_version=p_version;
  SELECT min(x) INTO healthy_at FROM (SELECT old_healthy x UNION ALL SELECT occurred_at FROM oauth_observation_events WHERE episode_account_id=p_episode AND event_type='probe_result' AND payload->>'model'=p_model AND payload->>'protocol'=p_protocol AND payload->>'probe_version'=p_version AND payload->>'verdict'='healthy') q;
  IF healthy_at IS NULL THEN RETURN; END IF;
  SELECT min(x) INTO degraded_at FROM (SELECT old_degraded x WHERE old_degraded > healthy_at UNION ALL SELECT occurred_at FROM oauth_observation_events WHERE episode_account_id=p_episode AND event_type='probe_result' AND payload->>'model'=p_model AND payload->>'protocol'=p_protocol AND payload->>'probe_version'=p_version AND payload->>'verdict'='degraded' AND occurred_at > healthy_at) q;
  INSERT INTO oauth_observation_probe_lifetimes VALUES (p_episode,p_model,p_protocol,p_version,healthy_at,degraded_at,clock_timestamp())
  ON CONFLICT (episode_account_id,model,protocol,probe_version) DO UPDATE SET first_healthy_at=EXCLUDED.first_healthy_at,first_degraded_at=EXCLUDED.first_degraded_at,updated_at=EXCLUDED.updated_at;
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_append(p_account_id BIGINT, p_event_key TEXT, p_occurred_at TIMESTAMPTZ, p_event_type TEXT, p_payload JSONB)
RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE episode BIGINT; clean JSONB; inserted_rows INTEGER; slot TEXT;
BEGIN
    IF p_event_key IS NULL OR length(p_event_key) = 0 OR length(p_event_key) > 256 OR p_occurred_at IS NULL THEN RETURN; END IF;
    episode := oauth_observation_ensure_episode(p_account_id);
    IF episode IS NULL THEN RETURN; END IF; -- deleted accounts remain eligible only when a prior episode exists.
    IF p_event_type = 'probe_result' THEN
      clean := jsonb_strip_nulls(jsonb_build_object('model',p_payload->'model','protocol',p_payload->'protocol','probe_version',p_payload->'probe_version','verdict',p_payload->'verdict','failure',p_payload->'failure','latency_ms',p_payload->'latency_ms','started_at',p_payload->'started_at','finished_at',p_payload->'finished_at','source',p_payload->'source','mint_status',p_payload->'mint_status','continue_status',p_payload->'continue_status','rule_id',p_payload->'rule_id','round_id',p_payload->'round_id','trigger_source',p_payload->'trigger_source','program_version',p_payload->'program_version'));
      IF NULLIF(clean->>'model','') IS NULL OR clean->>'protocol' IS DISTINCT FROM 'native'
        OR COALESCE(clean->>'probe_version','') !~ '^[a-zA-Z0-9_-]{1,64}$'
        OR COALESCE(clean->>'verdict','') NOT IN ('healthy','degraded','inconclusive') THEN RETURN; END IF;
    ELSIF p_event_type IN ('slot_acquired','slot_refreshed','slot_released','slot_release_failed') THEN
      clean := jsonb_strip_nulls(jsonb_build_object('slot_id',p_payload->'slot_id','limit',p_payload->'limit','expires_at',p_payload->'expires_at','protocol',p_payload->'protocol','program_version',p_payload->'program_version'));
      IF clean->>'slot_id' IS NULL THEN RETURN; END IF;
    ELSIF p_event_type IN ('slot_denied','slot_acquire_failed') THEN clean := jsonb_strip_nulls(jsonb_build_object('limit',p_payload->'limit','program_version',p_payload->'program_version'));
    ELSIF p_event_type IN ('selected','wait_selected') THEN clean := jsonb_strip_nulls(jsonb_build_object('model',p_payload->'model','protocol',p_payload->'protocol','layer',p_payload->'layer','group_id',p_payload->'group_id','selected_account_id',p_payload->'selected_account_id','program_version',p_payload->'program_version'));
    ELSIF p_event_type = 'candidate_filtered' THEN clean := jsonb_strip_nulls(jsonb_build_object('model',p_payload->'model','protocol',p_payload->'protocol','group_id',p_payload->'group_id','error_class',p_payload->'error_class','limit',p_payload->'limit','program_version',p_payload->'program_version'));
    ELSIF p_event_type IN ('request_outcome','live_call_outcome','fallback') THEN clean := jsonb_strip_nulls(jsonb_build_object('model',p_payload->'model','protocol',p_payload->'protocol','success',p_payload->'success','first_token_ms',p_payload->'first_token_ms','http_status',p_payload->'http_status','error_class',p_payload->'error_class','excluded_count',p_payload->'excluded_count','selected_account_id',p_payload->'selected_account_id','group_id',p_payload->'group_id','program_version',p_payload->'program_version'));
    ELSIF p_event_type IN ('queue_enter','queue_leave') THEN clean := jsonb_strip_nulls(jsonb_build_object('wait_id',p_payload->'wait_id','limit',p_payload->'limit','queue_delta',p_payload->'queue_delta','program_version',p_payload->'program_version'));
    ELSE RETURN;
    END IF;
    clean:=clean||jsonb_strip_nulls(jsonb_build_object('attempt_id',p_payload->'attempt_id','slot_role',p_payload->'slot_role','model',p_payload->'model'));
    IF p_event_type='probe_result' OR p_event_type IN('slot_acquired','slot_released','slot_release_failed','slot_refreshed') THEN
      -- Serialize state projections across API/worker instances. Batch writers
      -- sort account IDs before calling this function to avoid crossed locks.
      PERFORM 1 FROM oauth_observation_episodes WHERE account_id=episode FOR UPDATE;
    END IF;
    INSERT INTO oauth_observation_events(event_key,account_id,episode_account_id,occurred_at,event_type,payload)
      VALUES (p_event_key,p_account_id,episode,p_occurred_at,p_event_type,clean) ON CONFLICT (event_key) DO NOTHING;
    GET DIAGNOSTICS inserted_rows = ROW_COUNT;
    IF inserted_rows = 0 THEN RETURN; END IF;
    IF p_event_type = 'probe_result' THEN
      UPDATE oauth_observation_episodes SET last_probe_at=GREATEST(last_probe_at,p_occurred_at) WHERE account_id=episode;
      PERFORM oauth_observation_recompute_lifetime(episode, clean->>'model', clean->>'protocol', clean->>'probe_version');
    ELSIF p_event_type = 'slot_acquired' THEN
      INSERT INTO oauth_observation_slots(event_key,account_id,episode_account_id,slot_id,acquired_at,slot_limit,expires_at,protocol,attempt_id,slot_role,model)
      VALUES (p_event_key,p_account_id,episode,clean->>'slot_id',p_occurred_at,NULLIF(clean->>'limit','')::INTEGER,NULLIF(clean->>'expires_at','')::TIMESTAMPTZ,COALESCE(clean->>'protocol','unknown'),clean->>'attempt_id',clean->>'slot_role',clean->>'model')
      ON CONFLICT DO NOTHING;
      UPDATE oauth_observation_slots s SET released_at=(SELECT min(e.occurred_at) FROM oauth_observation_events e WHERE e.episode_account_id=episode AND e.event_type='slot_released' AND e.payload->>'slot_id'=s.slot_id AND e.occurred_at>=s.acquired_at),release_state='released' WHERE s.event_key=p_event_key AND EXISTS (SELECT 1 FROM oauth_observation_events e WHERE e.episode_account_id=episode AND e.event_type='slot_released' AND e.payload->>'slot_id'=s.slot_id AND e.occurred_at>=s.acquired_at);
      UPDATE oauth_observation_slots s SET expires_at=GREATEST(s.expires_at,(SELECT max((e.payload->>'expires_at')::timestamptz) FROM oauth_observation_events e WHERE e.episode_account_id=episode AND e.event_type='slot_refreshed' AND e.payload->>'slot_id'=s.slot_id AND e.occurred_at>=s.acquired_at)) WHERE s.event_key=p_event_key;
      UPDATE oauth_observation_slots s SET release_state='release_failed' WHERE s.event_key=p_event_key AND released_at IS NULL AND EXISTS(SELECT 1 FROM oauth_observation_events e WHERE e.episode_account_id=episode AND e.event_type='slot_release_failed' AND e.payload->>'slot_id'=s.slot_id AND e.occurred_at>=s.acquired_at);
    ELSIF p_event_type = 'slot_refreshed' THEN
      UPDATE oauth_observation_slots SET expires_at=GREATEST(COALESCE(expires_at,'-infinity'::timestamptz),NULLIF(clean->>'expires_at','')::timestamptz),slot_limit=COALESCE(NULLIF(clean->>'limit','')::INTEGER,slot_limit),protocol=COALESCE(clean->>'protocol',protocol) WHERE episode_account_id=episode AND slot_id=clean->>'slot_id';
    ELSIF p_event_type = 'slot_released' THEN
      slot := clean->>'slot_id';
      UPDATE oauth_observation_slots SET released_at = LEAST(released_at, p_occurred_at),
          release_state = 'released'
        WHERE episode_account_id=episode AND slot_id=slot AND acquired_at<=p_occurred_at;
    ELSIF p_event_type='slot_release_failed' THEN
      UPDATE oauth_observation_slots SET release_state='release_failed' WHERE episode_account_id=episode AND slot_id=clean->>'slot_id' AND released_at IS NULL;
    END IF;
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_account_snapshot(r JSONB)
RETURNS JSONB LANGUAGE sql IMMUTABLE SET search_path=pg_catalog,public,pg_temp AS $$
 SELECT jsonb_strip_nulls(jsonb_build_object(
 'priority',r->'priority',
 'concurrency',r->'concurrency',
 'load_factor',r->'load_factor',
 'status',r->'status',
 'schedulable',r->'schedulable',
 'expires_at',r->'expires_at',
 'auto_pause_on_expired',r->'auto_pause_on_expired',
 'proxy_id',r->'proxy_id',
 'deleted_at',r->'deleted_at',
 'rate_limited_at',r->'rate_limited_at',
 'rate_limit_reset_at',r->'rate_limit_reset_at',
 'overload_until',r->'overload_until',
 'temp_unschedulable_until',r->'temp_unschedulable_until',
 'group_rate_multiplier',r->'group_rate_multiplier',
 'plan_type',r->'credentials'->'plan_type','credential_expiry',r->'credentials'->'expires_at',
 'bps_enabled',r->'extra'->'openai_excel_bps','bps_models',r->'extra'->'openai_excel_bps_models',
 'extra',jsonb_build_object('openai_excel_bps',r->'extra'->'openai_excel_bps',
 'openai_excel_bps_models',r->'extra'->'openai_excel_bps_models',
 'openai_excel_bps_auto_disable_on_403',r->'extra'->'openai_excel_bps_auto_disable_on_403',
 'openai_excel_bps_auto_recover_on_403',r->'extra'->'openai_excel_bps_auto_recover_on_403',
 'openai_excel_bps_403_disabled_at',r->'extra'->'openai_excel_bps_403_disabled_at',
 'codex_5h_used_percent',r->'extra'->'codex_5h_used_percent',
 'codex_7d_used_percent',r->'extra'->'codex_7d_used_percent',
 'codex_5h_reset_at',r->'extra'->'codex_5h_reset_at',
 'codex_7d_reset_at',r->'extra'->'codex_7d_reset_at',
 'codex_usage_updated_at',r->'extra'->'codex_usage_updated_at',
 'codex_primary_used_percent',r->'extra'->'codex_primary_used_percent',
 'codex_primary_window_minutes',r->'extra'->'codex_primary_window_minutes',
 'codex_primary_reset_at',r->'extra'->'codex_primary_reset_at',
 'codex_secondary_used_percent',r->'extra'->'codex_secondary_used_percent',
 'codex_secondary_window_minutes',r->'extra'->'codex_secondary_window_minutes',
 'codex_secondary_reset_at',r->'extra'->'codex_secondary_reset_at')))
$$;

CREATE OR REPLACE FUNCTION oauth_observation_provenance()
RETURNS JSONB LANGUAGE sql STABLE SET search_path=pg_catalog,public,pg_temp AS $$
 SELECT jsonb_strip_nulls(jsonb_build_object('schema_version','263',
 'source',COALESCE(NULLIF(current_setting('oauth_observation.source',true),''),'unknown'),
 'rule_id',NULLIF(current_setting('oauth_observation.rule_id',true),''),
 'quality_outcome',NULLIF(current_setting('oauth_observation.outcome',true),''),
 'bps_trigger',NULLIF(current_setting('oauth_observation.bps_trigger',true),'')))
$$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_account()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE r JSONB; previous JSONB; following JSONB; credentials_changed BOOLEAN:=FALSE;
BEGIN
 r:=to_jsonb(CASE WHEN TG_OP='DELETE' THEN OLD ELSE NEW END);
 IF r->>'platform'<>'openai' OR r->>'type'<>'oauth' THEN RETURN COALESCE(NEW,OLD); END IF;
 IF TG_OP<>'INSERT' THEN previous:=oauth_observation_account_snapshot(to_jsonb(OLD)); END IF;
 IF TG_OP<>'DELETE' THEN following:=oauth_observation_account_snapshot(to_jsonb(NEW)); END IF;
 IF TG_OP='UPDATE' THEN
   credentials_changed := OLD.credentials->'access_token' IS DISTINCT FROM NEW.credentials->'access_token'
                       OR OLD.credentials->'refresh_token' IS DISTINCT FROM NEW.credentials->'refresh_token';
 END IF;
 IF previous IS DISTINCT FROM following OR credentials_changed THEN
   -- BEFORE DELETE keeps identity available even with no prior observations.
   PERFORM oauth_observation_ensure_episode((r->>'id')::BIGINT, TG_OP<>'DELETE');
   INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot)
   VALUES('account',r->>'id',lower(TG_OP),oauth_observation_provenance() ||
     jsonb_build_object('account_id',r->'id','before',previous,'after',following,'credentials_changed',credentials_changed));
 END IF;
 RETURN COALESCE(NEW,OLD);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_account_group()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE r JSONB:=to_jsonb(CASE WHEN TG_OP='DELETE' THEN OLD ELSE NEW END); before_snap JSONB; after_snap JSONB;
BEGIN
 IF oauth_observation_ensure_episode((r->>'account_id')::BIGINT) IS NULL THEN RETURN COALESCE(NEW,OLD); END IF;
 IF TG_OP<>'INSERT' THEN before_snap:=jsonb_build_object('account_id',OLD.account_id,'group_id',OLD.group_id,'priority',OLD.priority,'allowed_models',to_jsonb(OLD)->'allowed_models'); END IF;
 IF TG_OP<>'DELETE' THEN after_snap:=jsonb_build_object('account_id',NEW.account_id,'group_id',NEW.group_id,'priority',NEW.priority,'allowed_models',to_jsonb(NEW)->'allowed_models'); END IF;
 IF before_snap IS DISTINCT FROM after_snap THEN
 INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot)
 VALUES('account_group',(r->>'account_id')||':'||(r->>'group_id'),lower(TG_OP),
 oauth_observation_provenance()||jsonb_build_object('account_id',r->'account_id','before',before_snap,'after',after_snap));
 END IF;
 RETURN COALESCE(NEW,OLD);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_rule_snapshot(r JSONB)
RETURNS JSONB LANGUAGE sql IMMUTABLE SET search_path=pg_catalog,public,pg_temp AS $$
 SELECT jsonb_strip_nulls(jsonb_build_object('account_id',r->'account_id','model_id',r->'model_id',
 'cron_expression',r->'cron_expression','enabled',r->'enabled','max_results',r->'max_results',
 'question_kind',r->'pelican_config'->'question_kind','test_channel',r->'pelican_config'->'test_channel',
 'parallel_count',r->'pelican_config'->'parallel_count','reasoning_effort',r->'pelican_config'->'reasoning_effort',
 'quality',jsonb_build_object('action',r->'pelican_config'->'quality'->'action',
 'auto_restore',r->'pelican_config'->'quality'->'auto_restore',
 'remove_group_ids',r->'pelican_config'->'quality'->'remove_group_ids',
 'trigger_on_upstream_5xx',r->'pelican_config'->'quality'->'trigger_on_upstream_5xx',
 'bps',jsonb_build_object('failure_threshold',r->'pelican_config'->'quality'->'bps'->'failure_threshold',
 'usage_percent',r->'pelican_config'->'quality'->'bps'->'usage_percent',
 'require_all',r->'pelican_config'->'quality'->'bps'->'require_all',
 'pass_threshold',r->'pelican_config'->'quality'->'bps'->'pass_threshold',
 'hold_on_usage',r->'pelican_config'->'quality'->'bps'->'hold_on_usage',
 'all_models',r->'pelican_config'->'quality'->'bps'->'all_models',
 'models',r->'pelican_config'->'quality'->'bps'->'models'))))
$$;
CREATE OR REPLACE FUNCTION oauth_observation_archive_plan()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE r JSONB:=to_jsonb(CASE WHEN TG_OP='DELETE' THEN OLD ELSE NEW END); b JSONB; a JSONB; version TEXT;
BEGIN
 IF oauth_observation_ensure_episode((r->>'account_id')::BIGINT) IS NULL THEN RETURN COALESCE(NEW,OLD); END IF;
 IF TG_OP<>'INSERT' THEN b:=oauth_observation_rule_snapshot(to_jsonb(OLD)); END IF;
 IF TG_OP<>'DELETE' THEN a:=oauth_observation_rule_snapshot(to_jsonb(NEW)); END IF;
 IF b IS NOT DISTINCT FROM a AND (TG_OP<>'UPDATE' OR OLD.pelican_config IS NOT DISTINCT FROM NEW.pelican_config) THEN RETURN COALESCE(NEW,OLD); END IF;
 -- The digest includes sensitive prompt/config changes but never stores their contents.
 SELECT encode(hmac(convert_to(COALESCE(r->'pelican_config','{}'::jsonb)::text,'UTF8'),secret,'sha256'),'hex')
 INTO version FROM oauth_observation_identity_secrets WHERE id;
 INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot)
 VALUES('scheduled_test_plan',r->>'id',lower(TG_OP),oauth_observation_provenance()||
 jsonb_build_object('account_id',r->'account_id','rule_version',version,'before',b,'after',a));
 RETURN COALESCE(NEW,OLD);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_result()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE r JSONB := to_jsonb(CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END); account BIGINT;
BEGIN
  SELECT account_id INTO account FROM scheduled_test_plans WHERE id=(r->>'plan_id')::BIGINT;
  IF account IS NULL THEN SELECT (snapshot->>'account_id')::BIGINT INTO account FROM oauth_observation_archives WHERE subject_type='scheduled_test_plan' AND subject_key=r->>'plan_id' ORDER BY id DESC LIMIT 1; END IF;
  IF account IS NULL OR (NOT oauth_observation_account_is_eligible(account) AND NOT EXISTS (SELECT 1 FROM oauth_observation_episodes WHERE account_id=account)) THEN RETURN COALESCE(NEW,OLD); END IF;
  INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot) VALUES ('scheduled_test_result',r->>'id',lower(TG_OP),jsonb_strip_nulls(jsonb_build_object('source',COALESCE(NULLIF(current_setting('oauth_observation.source',true),''),'unknown'),'rule_id',NULLIF(current_setting('oauth_observation.rule_id',true),''),'quality_outcome',NULLIF(current_setting('oauth_observation.outcome',true),''),'bps_trigger',NULLIF(current_setting('oauth_observation.bps_trigger',true),''),'plan_id',r->'plan_id','account_id',to_jsonb(account),'status',r->'status','latency_ms',r->'latency_ms','started_at',r->'started_at','finished_at',r->'finished_at','quality_action',r->'quality_action','verdict',r->'quality_judgment'->'verdict')));
  RETURN COALESCE(NEW,OLD);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_audit_mutation()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE account BIGINT; row JSONB;
BEGIN
  IF COALESCE(NEW.extra->'params'->>'id','') !~ '^[0-9]{1,18}$' THEN RETURN NEW; END IF;
  account := (NEW.extra->'params'->>'id')::BIGINT;
  IF NEW.status_code < 200 OR NEW.status_code >= 300 OR NEW.method NOT IN ('POST','PUT','PATCH','DELETE')
     OR NEW.actor_role NOT IN ('admin','super_admin') OR NEW.actor_user_id IS NULL
     OR NEW.action NOT LIKE 'admin.accounts.%' OR account IS NULL THEN RETURN NEW; END IF;
  IF NOT oauth_observation_account_is_eligible(account) AND NOT EXISTS (SELECT 1 FROM oauth_observation_episodes WHERE account_id=account) THEN RETURN NEW; END IF;
  SELECT to_jsonb(a) INTO row FROM accounts a WHERE a.id=account;
  INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot) VALUES
    ('admin_account_mutation',account::TEXT,'insert',jsonb_strip_nulls(jsonb_build_object(
      'actor_type',NEW.actor_role,'actor_id',NEW.actor_user_id,'action',NEW.action,'reason','admin_api','observation_kind','post_hoc_audit_observed',
      'priority',row->'priority','concurrency',row->'concurrency','load_factor',row->'load_factor','status',row->'status','schedulable',row->'schedulable','expires_at',row->'expires_at','proxy_id',row->'proxy_id',
      'account_snapshot',oauth_observation_account_snapshot(row))));
  RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_usage_apply(p_row JSONB,p_sign BIGINT)
RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE aid BIGINT:=(p_row->>'account_id')::BIGINT; uid BIGINT:=(p_row->>'id')::BIGINT;
 item oauth_observation_usage_contributions%ROWTYPE; sign BIGINT:=p_sign;
BEGIN
 IF p_sign<0 THEN
   SELECT * INTO item FROM oauth_observation_usage_contributions WHERE usage_log_id=uid FOR UPDATE;
   IF NOT FOUND THEN RETURN; END IF;
   DELETE FROM oauth_observation_usage_contributions WHERE usage_log_id=uid;
 ELSE
   IF oauth_observation_ensure_episode(aid) IS NULL THEN RETURN; END IF;
   IF (p_row->>'created_at')::timestamptz<clock_timestamp()-interval '180 days' THEN RETURN; END IF;
   item.usage_log_id:=uid; item.account_id:=aid;
   item.occurred_at:=(p_row->>'created_at')::timestamptz;
   item.minute_at:=date_trunc('minute',(p_row->>'created_at')::timestamptz);
   item.model:=COALESCE(NULLIF(p_row->>'upstream_model',''),p_row->>'model');
   item.protocol:=CASE
     WHEN p_row->>'upstream_endpoint' LIKE '/basispoints/%' THEN 'bps'
     WHEN p_row->>'upstream_endpoint' LIKE '%/live%' THEN 'live'
     WHEN COALESCE(p_row->>'upstream_endpoint','')='' THEN 'unknown' ELSE 'native' END;
   item.input_tokens:=COALESCE((p_row->>'input_tokens')::bigint,0);
   item.output_tokens:=COALESCE((p_row->>'output_tokens')::bigint,0);
   item.cache_creation_tokens:=COALESCE((p_row->>'cache_creation_tokens')::bigint,0);
   item.cache_read_tokens:=COALESCE((p_row->>'cache_read_tokens')::bigint,0);
   item.total_cost:=COALESCE((p_row->>'total_cost')::numeric,0);
   item.actual_cost:=COALESCE((p_row->>'actual_cost')::numeric,0);
   item.duration_ms:=NULLIF(p_row->>'duration_ms','')::bigint;
   IF item.duration_ms<=0 THEN item.duration_ms:=NULL; END IF;
   item.first_token_ms:=NULLIF(p_row->>'first_token_ms','')::bigint;
   item.reasoning_effort:=p_row->>'reasoning_effort';item.service_tier:=p_row->>'service_tier';
   item.long_context_billing_applied:=COALESCE((p_row->>'long_context_billing_applied')::boolean,false);
   item.usage_completeness:=COALESCE(NULLIF(p_row->>'usage_completeness',''),'unknown');
   INSERT INTO oauth_observation_usage_contributions SELECT item.* ON CONFLICT(usage_log_id) DO NOTHING;
   IF NOT FOUND THEN RETURN; END IF;
 END IF;
 INSERT INTO oauth_observation_usage_minutes(account_id,minute_at,model,protocol,requests,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,total_cost,actual_cost,latency_count,latency_ms_total,first_token_count,first_token_ms_total,complete_requests,partial_requests,unknown_requests)
 VALUES(item.account_id,item.minute_at,item.model,item.protocol,sign,sign*item.input_tokens,sign*item.output_tokens,
 sign*item.cache_creation_tokens,sign*item.cache_read_tokens,sign*item.total_cost,sign*item.actual_cost,
 sign*CASE WHEN item.duration_ms IS NULL THEN 0 ELSE 1 END,sign*COALESCE(item.duration_ms,0),
 sign*CASE WHEN item.first_token_ms IS NULL THEN 0 ELSE 1 END,sign*COALESCE(item.first_token_ms,0),
 sign*CASE WHEN item.usage_completeness='complete' THEN 1 ELSE 0 END,
 sign*CASE WHEN item.usage_completeness='partial' THEN 1 ELSE 0 END,
 sign*CASE WHEN item.usage_completeness NOT IN('complete','partial') THEN 1 ELSE 0 END)
 ON CONFLICT(account_id,minute_at,model,protocol) DO UPDATE SET
 requests=oauth_observation_usage_minutes.requests+EXCLUDED.requests,
 input_tokens=oauth_observation_usage_minutes.input_tokens+EXCLUDED.input_tokens,
 output_tokens=oauth_observation_usage_minutes.output_tokens+EXCLUDED.output_tokens,
 cache_creation_tokens=oauth_observation_usage_minutes.cache_creation_tokens+EXCLUDED.cache_creation_tokens,
 cache_read_tokens=oauth_observation_usage_minutes.cache_read_tokens+EXCLUDED.cache_read_tokens,
 total_cost=oauth_observation_usage_minutes.total_cost+EXCLUDED.total_cost,
 actual_cost=oauth_observation_usage_minutes.actual_cost+EXCLUDED.actual_cost,
 latency_count=oauth_observation_usage_minutes.latency_count+EXCLUDED.latency_count,
 latency_ms_total=oauth_observation_usage_minutes.latency_ms_total+EXCLUDED.latency_ms_total,
 first_token_count=oauth_observation_usage_minutes.first_token_count+EXCLUDED.first_token_count,
 first_token_ms_total=oauth_observation_usage_minutes.first_token_ms_total+EXCLUDED.first_token_ms_total,
 complete_requests=oauth_observation_usage_minutes.complete_requests+EXCLUDED.complete_requests,
 partial_requests=oauth_observation_usage_minutes.partial_requests+EXCLUDED.partial_requests,
 unknown_requests=oauth_observation_usage_minutes.unknown_requests+EXCLUDED.unknown_requests;
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_usage_trigger() RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$ BEGIN IF TG_OP='UPDATE' THEN PERFORM oauth_observation_usage_apply(to_jsonb(OLD),-1); END IF; IF TG_OP <> 'DELETE' THEN PERFORM oauth_observation_usage_apply(to_jsonb(NEW),1); END IF; RETURN COALESCE(NEW,OLD); END $$;

DROP TRIGGER IF EXISTS oauth_observation_accounts_archive ON accounts;
CREATE TRIGGER oauth_observation_accounts_archive AFTER INSERT OR UPDATE ON accounts FOR EACH ROW EXECUTE FUNCTION oauth_observation_archive_account();
DROP TRIGGER IF EXISTS oauth_observation_accounts_delete_archive ON accounts;
CREATE TRIGGER oauth_observation_accounts_delete_archive BEFORE DELETE ON accounts FOR EACH ROW EXECUTE FUNCTION oauth_observation_archive_account();
DROP TRIGGER IF EXISTS oauth_observation_account_groups_archive ON account_groups;
CREATE TRIGGER oauth_observation_account_groups_archive AFTER INSERT OR UPDATE OR DELETE ON account_groups FOR EACH ROW EXECUTE FUNCTION oauth_observation_archive_account_group();
DROP TRIGGER IF EXISTS oauth_observation_plans_archive ON scheduled_test_plans;
CREATE TRIGGER oauth_observation_plans_archive AFTER INSERT OR UPDATE OR DELETE ON scheduled_test_plans FOR EACH ROW EXECUTE FUNCTION oauth_observation_archive_plan();
DROP TRIGGER IF EXISTS oauth_observation_results_archive ON scheduled_test_results;
CREATE TRIGGER oauth_observation_results_archive AFTER INSERT OR UPDATE OR DELETE ON scheduled_test_results FOR EACH ROW EXECUTE FUNCTION oauth_observation_archive_result();
DROP TRIGGER IF EXISTS oauth_observation_usage_minutes ON usage_logs;
CREATE TRIGGER oauth_observation_usage_minutes AFTER INSERT OR UPDATE OR DELETE ON usage_logs FOR EACH ROW EXECUTE FUNCTION oauth_observation_usage_trigger();
DROP TRIGGER IF EXISTS oauth_observation_audit_mutation_archive ON audit_logs;
CREATE TRIGGER oauth_observation_audit_mutation_archive AFTER INSERT ON audit_logs FOR EACH ROW EXECUTE FUNCTION oauth_observation_archive_audit_mutation();

CREATE OR REPLACE FUNCTION oauth_observation_prune() RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
  DELETE FROM oauth_observation_events WHERE id IN (SELECT id FROM oauth_observation_events WHERE recorded_at < clock_timestamp()-INTERVAL '90 days' ORDER BY id LIMIT 5000);
  DELETE FROM oauth_observation_slots WHERE id IN (SELECT id FROM oauth_observation_slots WHERE recorded_at < clock_timestamp()-INTERVAL '90 days' ORDER BY id LIMIT 5000);
  DELETE FROM oauth_observation_usage_contributions WHERE usage_log_id IN (SELECT usage_log_id FROM oauth_observation_usage_contributions WHERE minute_at < clock_timestamp()-INTERVAL '180 days' ORDER BY usage_log_id LIMIT 5000);
  DELETE FROM oauth_observation_usage_minutes WHERE ctid IN (SELECT ctid FROM oauth_observation_usage_minutes WHERE minute_at < clock_timestamp()-INTERVAL '180 days' LIMIT 5000);
  DELETE FROM oauth_observation_recorder_health WHERE instance_id IN (SELECT instance_id FROM oauth_observation_recorder_health WHERE observed_at < clock_timestamp()-INTERVAL '90 days' LIMIT 5000);
  DELETE FROM oauth_observation_archives WHERE id IN (SELECT id FROM oauth_observation_archives WHERE observed_at < clock_timestamp()-INTERVAL '90 days' ORDER BY id LIMIT 5000);
  DELETE FROM oauth_observation_recorder_health_minutes WHERE ctid IN (SELECT ctid FROM oauth_observation_recorder_health_minutes WHERE minute_at<clock_timestamp()-interval '90 days' LIMIT 5000);
END $$;

-- Split slot intervals across actual minute boundaries, then sweep simultaneous
-- starts/ends together. Long requests are never charged wholly to their start
-- minute. Open/expired/failed-release observations remain explicitly incomplete.
CREATE OR REPLACE FUNCTION oauth_observation_occupancy(p_from TIMESTAMPTZ,p_to TIMESTAMPTZ,p_account BIGINT DEFAULT NULL)
RETURNS TABLE(minute_at TIMESTAMPTZ,episode_account_id BIGINT,occupied_seconds NUMERIC,
 peak_concurrency BIGINT,average_concurrency NUMERIC,incomplete_slots BIGINT,
 observed_limit INTEGER,full_capacity_seconds NUMERIC)
LANGUAGE plpgsql STABLE SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF p_to<=p_from OR p_to-p_from>interval '7 days' THEN RAISE EXCEPTION 'occupancy window must be positive and at most seven days'; END IF;
 RETURN QUERY
 WITH clipped AS(
 SELECT s.episode_account_id aid,GREATEST(s.acquired_at,p_from) a,
 LEAST(COALESCE(s.released_at,s.expires_at,p_to),COALESCE(s.expires_at,p_to),p_to) b,
 (s.released_at IS NULL OR s.release_state<>'released' OR s.released_at>s.expires_at) uncertain,s.slot_limit lim
 FROM oauth_observation_slots s
 WHERE (p_account IS NULL OR s.episode_account_id=p_account)
 AND s.acquired_at<p_to AND COALESCE(s.released_at,s.expires_at,p_to)>p_from
 AND COALESCE(s.slot_role,'request')<>'live_admission'
 ), pieces AS(
 SELECT c.aid,m AS m,GREATEST(c.a,m) a,LEAST(c.b,m+interval '1 minute') b,c.uncertain,c.lim
 FROM clipped c CROSS JOIN LATERAL generate_series(date_trunc('minute',c.a),date_trunc('minute',c.b-interval '1 microsecond'),interval '1 minute') m
 WHERE c.b>c.a
 ), bounds AS(
 SELECT p.aid,p.m,count(*) FILTER(WHERE p.uncertain) missing,
 CASE WHEN min(p.lim)=max(p.lim) AND count(p.lim)=count(*) THEN min(p.lim) END lim
 FROM pieces p GROUP BY p.aid,p.m
 ), points AS(
 SELECT p.aid,p.m,p.a t,1::bigint delta FROM pieces p UNION ALL
 SELECT p.aid,p.m,p.b,-1::bigint FROM pieces p
 ), grouped AS(SELECT aid,m,t,sum(delta) delta FROM points GROUP BY aid,m,t),
 swept AS(SELECT aid,m,t,lead(t) OVER(PARTITION BY aid,m ORDER BY t) next_t,
 sum(delta) OVER(PARTITION BY aid,m ORDER BY t) active FROM grouped)
 SELECT s.m,s.aid,sum(s.active*extract(epoch FROM(s.next_t-s.t))),
 CASE WHEN b.missing=0 THEN max(s.active)::bigint END,
 CASE WHEN b.missing=0 THEN sum(s.active*extract(epoch FROM(s.next_t-s.t)))/extract(epoch FROM(LEAST(s.m+interval '1 minute',p_to)-GREATEST(s.m,p_from))) END,
 b.missing,b.lim,
 CASE WHEN b.missing=0 AND b.lim>0 THEN COALESCE(sum(extract(epoch FROM(s.next_t-s.t))) FILTER(WHERE s.active>=b.lim),0) END
 FROM swept s JOIN bounds b ON b.aid=s.aid AND b.m=s.m WHERE s.next_t>s.t
 GROUP BY s.m,s.aid,b.missing,b.lim ORDER BY s.m,s.aid;
END $$;

-- Capture native credential-guard outcomes without account names or diagnostic
-- text. These observations NEVER update the probe lifetime projections.
CREATE OR REPLACE FUNCTION oauth_observation_archive_guard()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
BEGIN
 IF oauth_observation_ensure_episode(NEW.account_id) IS NULL THEN RETURN NEW; END IF;
 INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot,observed_at)
 VALUES('credential_guard',NEW.id::text,'insert',jsonb_build_object('account_id',NEW.account_id,
 'kind',CASE WHEN NEW.kind IN('probe_ok','probe_auth','probe_transient','relogin_ok','relogin_failed','state_fixed','state_failed','manual_run') THEN NEW.kind ELSE 'other' END,
 'latency_ms',NEW.latency_ms,'source','credential_guard'),NEW.created_at);
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_settings()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE r JSONB:=to_jsonb(CASE WHEN TG_OP='DELETE' THEN OLD ELSE NEW END); b TEXT; a TEXT;
BEGIN
 IF r->>'key' NOT IN('openai_advanced_scheduler_enabled','openai_advanced_scheduler_subscription_priority_enabled',
 'openai_advanced_scheduler_sticky_weighted_enabled','openai_advanced_scheduler_lb_top_k',
 'openai_advanced_scheduler_weight_priority','openai_advanced_scheduler_weight_load',
 'openai_advanced_scheduler_weight_queue','openai_advanced_scheduler_weight_error_rate',
 'openai_advanced_scheduler_weight_ttft','openai_advanced_scheduler_weight_reset',
 'openai_advanced_scheduler_weight_quota_headroom','openai_advanced_scheduler_weight_upstream_cost',
 'openai_advanced_scheduler_weight_previous_response','openai_advanced_scheduler_weight_session_sticky',
 'openai_low_upstream_rate_priority_enabled','openai_oauth_scheduling_rate_multiplier') THEN RETURN COALESCE(NEW,OLD); END IF;
 IF TG_OP<>'INSERT' THEN b:=OLD.value; END IF;
 IF TG_OP<>'DELETE' THEN a:=NEW.value; END IF;
 IF a IS NOT DISTINCT FROM b THEN RETURN COALESCE(NEW,OLD); END IF;
 INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot)
 VALUES('scheduler_settings',r->>'key',lower(TG_OP),jsonb_build_object('key',r->>'key','before',b,'after',a));
 RETURN COALESCE(NEW,OLD);
END $$;
DO $$ BEGIN
 IF to_regclass('public.account_token_guard_events') IS NOT NULL THEN
   DROP TRIGGER IF EXISTS oauth_observation_guard_archive ON account_token_guard_events;
   CREATE TRIGGER oauth_observation_guard_archive AFTER INSERT ON account_token_guard_events FOR EACH ROW EXECUTE FUNCTION oauth_observation_archive_guard();
 END IF;
 IF to_regclass('public.settings') IS NOT NULL THEN
   DROP TRIGGER IF EXISTS oauth_observation_settings_archive ON settings;
   CREATE TRIGGER oauth_observation_settings_archive AFTER INSERT OR UPDATE OR DELETE ON settings FOR EACH ROW EXECUTE FUNCTION oauth_observation_archive_settings();
 END IF;
END $$;

-- Application migrations and runtime use the owning DB role. Lower-privilege
-- readers must not be able to manufacture observations via definer functions.
REVOKE ALL ON FUNCTION oauth_observation_ensure_episode(BIGINT,BOOLEAN) FROM PUBLIC;
REVOKE ALL ON FUNCTION oauth_observation_append(BIGINT,TEXT,TIMESTAMPTZ,TEXT,JSONB) FROM PUBLIC;
REVOKE ALL ON FUNCTION oauth_observation_recompute_lifetime(BIGINT,TEXT,TEXT,TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION oauth_observation_usage_apply(JSONB,BIGINT) FROM PUBLIC;
REVOKE ALL ON FUNCTION oauth_observation_prune() FROM PUBLIC;

COMMENT ON TABLE oauth_observation_probe_lifetimes IS 'First healthy native turn-state probe through first later degraded probe; inconclusive, expiry, quota and scheduling transitions never terminate it.';
COMMENT ON TABLE oauth_observation_episode_lifetimes IS 'Primary account episode lifetime across all native probe model/version series; detailed series remain in oauth_observation_probe_lifetimes.';
COMMENT ON TABLE oauth_observation_archives IS 'Whitelisted immutable operational snapshots; deliberately has no foreign keys or raw response/error text.';
