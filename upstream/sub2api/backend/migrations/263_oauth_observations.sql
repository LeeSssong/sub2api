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
    acquired_at TIMESTAMPTZ NOT NULL,
    released_at TIMESTAMPTZ,
    release_state TEXT NOT NULL DEFAULT 'open' CHECK (release_state IN ('open', 'released', 'release_failed')),
    slot_limit INTEGER,
    expires_at TIMESTAMPTZ,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_slots_episode_acquired ON oauth_observation_slots(episode_account_id, acquired_at);
CREATE INDEX IF NOT EXISTS idx_oauth_observation_slots_open ON oauth_observation_slots(acquired_at) WHERE released_at IS NULL;

CREATE TABLE IF NOT EXISTS oauth_observation_usage_contributions (
    usage_log_id BIGINT PRIMARY KEY,
    account_id BIGINT NOT NULL,
    minute_at TIMESTAMPTZ NOT NULL,
    model TEXT NOT NULL,
    protocol TEXT NOT NULL,
    input_tokens BIGINT NOT NULL,
    output_tokens BIGINT NOT NULL,
    cache_creation_tokens BIGINT NOT NULL,
    cache_read_tokens BIGINT NOT NULL,
    actual_cost NUMERIC(20,10) NOT NULL,
    duration_ms BIGINT,
    first_token_ms BIGINT
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
    latency_count BIGINT NOT NULL DEFAULT 0,
    latency_ms_total BIGINT NOT NULL DEFAULT 0,
    first_token_count BIGINT NOT NULL DEFAULT 0,
    first_token_ms_total BIGINT NOT NULL DEFAULT 0,
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

CREATE TABLE IF NOT EXISTS oauth_observation_recorder_health (
    instance_id TEXT PRIMARY KEY,
    observed_at TIMESTAMPTZ NOT NULL,
    enqueued BIGINT NOT NULL,
    persisted BIGINT NOT NULL,
    dropped BIGINT NOT NULL,
    write_errors BIGINT NOT NULL,
    queue_depth INTEGER NOT NULL
);

CREATE OR REPLACE FUNCTION oauth_observation_account_is_eligible(p_account_id BIGINT)
RETURNS BOOLEAN LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
      SELECT 1 FROM accounts a
      WHERE a.id = p_account_id AND lower(a.platform) = 'openai' AND lower(a.type) = 'oauth'
    )
$$;

CREATE OR REPLACE FUNCTION oauth_observation_ensure_episode(p_account_id BIGINT)
RETURNS BIGINT LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
DECLARE
    identity_value BYTEA;
    subject BIGINT;
    creds JSONB;
BEGIN
    SELECT credentials INTO creds FROM accounts WHERE id = p_account_id;
    IF FOUND THEN
        IF NOT oauth_observation_account_is_eligible(p_account_id) THEN RETURN NULL; END IF;
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
          ON CONFLICT (account_id) DO UPDATE SET subject_id = COALESCE(oauth_observation_episodes.subject_id, EXCLUDED.subject_id), last_observed_at = clock_timestamp();
    END IF;
    RETURN (SELECT account_id FROM oauth_observation_episodes WHERE account_id = p_account_id);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_recompute_lifetime(p_episode BIGINT, p_model TEXT, p_protocol TEXT, p_version TEXT)
RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
DECLARE healthy_at TIMESTAMPTZ; degraded_at TIMESTAMPTZ; global_healthy_at TIMESTAMPTZ; global_degraded_at TIMESTAMPTZ;
BEGIN
    SELECT min(occurred_at) INTO global_healthy_at FROM oauth_observation_events
     WHERE episode_account_id = p_episode AND event_type = 'probe_result' AND payload->>'verdict' = 'healthy';
    IF global_healthy_at IS NOT NULL THEN
      SELECT min(occurred_at) INTO global_degraded_at FROM oauth_observation_events
       WHERE episode_account_id = p_episode AND event_type = 'probe_result' AND payload->>'verdict' = 'degraded' AND occurred_at > global_healthy_at;
      INSERT INTO oauth_observation_episode_lifetimes(episode_account_id, first_healthy_at, first_degraded_at, updated_at)
        VALUES (p_episode, global_healthy_at, global_degraded_at, clock_timestamp())
        ON CONFLICT (episode_account_id) DO UPDATE SET first_healthy_at=EXCLUDED.first_healthy_at, first_degraded_at=EXCLUDED.first_degraded_at, updated_at=EXCLUDED.updated_at;
    END IF;
    SELECT min(occurred_at) INTO healthy_at FROM oauth_observation_events
     WHERE episode_account_id = p_episode AND event_type = 'probe_result'
       AND payload->>'model' = p_model AND payload->>'protocol' = p_protocol
       AND payload->>'probe_version' = p_version AND payload->>'verdict' = 'healthy';
    IF healthy_at IS NULL THEN RETURN; END IF;
    SELECT min(occurred_at) INTO degraded_at FROM oauth_observation_events
     WHERE episode_account_id = p_episode AND event_type = 'probe_result'
       AND payload->>'model' = p_model AND payload->>'protocol' = p_protocol
       AND payload->>'probe_version' = p_version AND payload->>'verdict' = 'degraded'
       AND occurred_at > healthy_at;
    INSERT INTO oauth_observation_probe_lifetimes(episode_account_id, model, protocol, probe_version, first_healthy_at, first_degraded_at, updated_at)
      VALUES (p_episode, p_model, p_protocol, p_version, healthy_at, degraded_at, clock_timestamp())
      ON CONFLICT (episode_account_id, model, protocol, probe_version) DO UPDATE
      SET first_healthy_at = EXCLUDED.first_healthy_at, first_degraded_at = EXCLUDED.first_degraded_at, updated_at = EXCLUDED.updated_at;
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_append(p_account_id BIGINT, p_event_key TEXT, p_occurred_at TIMESTAMPTZ, p_event_type TEXT, p_payload JSONB)
RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
DECLARE episode BIGINT; clean JSONB; inserted_rows INTEGER; slot TEXT;
BEGIN
    IF p_event_key IS NULL OR length(p_event_key) = 0 OR length(p_event_key) > 256 OR p_occurred_at IS NULL THEN RETURN; END IF;
    episode := oauth_observation_ensure_episode(p_account_id);
    IF episode IS NULL THEN RETURN; END IF; -- deleted accounts remain eligible only when a prior episode exists.
    IF p_event_type = 'probe_result' THEN
      clean := jsonb_strip_nulls(jsonb_build_object('model',p_payload->'model','protocol',p_payload->'protocol','probe_version',p_payload->'probe_version','verdict',p_payload->'verdict','failure',p_payload->'failure','latency_ms',p_payload->'latency_ms','started_at',p_payload->'started_at','finished_at',p_payload->'finished_at','source',p_payload->'source','mint_status',p_payload->'mint_status','continue_status',p_payload->'continue_status','program_version',p_payload->'program_version'));
      IF clean->>'model' IS NULL OR clean->>'protocol' <> 'native' OR clean->>'probe_version' <> 'turn_state_v1' OR clean->>'verdict' NOT IN ('healthy','degraded','inconclusive') THEN RETURN; END IF;
    ELSIF p_event_type IN ('slot_acquired','slot_released','slot_release_failed') THEN
      clean := jsonb_strip_nulls(jsonb_build_object('slot_id',p_payload->'slot_id','limit',p_payload->'limit','expires_at',p_payload->'expires_at','program_version',p_payload->'program_version'));
      IF clean->>'slot_id' IS NULL THEN RETURN; END IF;
    ELSIF p_event_type IN ('slot_denied','slot_acquire_failed') THEN clean := jsonb_strip_nulls(jsonb_build_object('limit',p_payload->'limit','program_version',p_payload->'program_version'));
    ELSIF p_event_type IN ('selected','wait_selected') THEN clean := jsonb_strip_nulls(jsonb_build_object('model',p_payload->'model','protocol',p_payload->'protocol','layer',p_payload->'layer','group_id',p_payload->'group_id','selected_account_id',p_payload->'selected_account_id','program_version',p_payload->'program_version'));
    ELSIF p_event_type IN ('request_outcome','fallback') THEN clean := jsonb_strip_nulls(jsonb_build_object('model',p_payload->'model','protocol',p_payload->'protocol','success',p_payload->'success','first_token_ms',p_payload->'first_token_ms','http_status',p_payload->'http_status','error_class',p_payload->'error_class','excluded_count',p_payload->'excluded_count','program_version',p_payload->'program_version'));
    ELSIF p_event_type IN ('queue_enter','queue_leave') THEN clean := jsonb_strip_nulls(jsonb_build_object('wait_id',p_payload->'wait_id','limit',p_payload->'limit','queue_delta',p_payload->'queue_delta','program_version',p_payload->'program_version'));
    ELSE RETURN;
    END IF;
    INSERT INTO oauth_observation_events(event_key,account_id,episode_account_id,occurred_at,event_type,payload)
      VALUES (p_event_key,p_account_id,episode,p_occurred_at,p_event_type,clean) ON CONFLICT (event_key) DO NOTHING;
    GET DIAGNOSTICS inserted_rows = ROW_COUNT;
    IF inserted_rows = 0 THEN RETURN; END IF;
    IF p_event_type = 'probe_result' THEN
      PERFORM oauth_observation_recompute_lifetime(episode, clean->>'model', clean->>'protocol', clean->>'probe_version');
    ELSIF p_event_type = 'slot_acquired' THEN
      INSERT INTO oauth_observation_slots(event_key,account_id,episode_account_id,slot_id,acquired_at,slot_limit,expires_at)
      VALUES (p_event_key,p_account_id,episode,clean->>'slot_id',p_occurred_at,NULLIF(clean->>'limit','')::INTEGER,NULLIF(clean->>'expires_at','')::TIMESTAMPTZ)
      ON CONFLICT (event_key) DO NOTHING;
      UPDATE oauth_observation_slots s SET released_at = GREATEST(s.acquired_at, e.occurred_at), release_state = CASE WHEN e.event_type = 'slot_released' THEN 'released' ELSE 'release_failed' END
      FROM oauth_observation_events e WHERE s.event_key = p_event_key AND e.episode_account_id = episode AND e.event_type IN ('slot_released','slot_release_failed') AND e.payload->>'slot_id' = s.slot_id AND e.occurred_at >= s.acquired_at;
    ELSIF p_event_type IN ('slot_released','slot_release_failed') THEN
      slot := clean->>'slot_id';
      UPDATE oauth_observation_slots SET released_at = GREATEST(acquired_at, p_occurred_at),
          release_state = CASE WHEN p_event_type = 'slot_released' THEN 'released' ELSE 'release_failed' END
        WHERE id = (SELECT id FROM oauth_observation_slots WHERE episode_account_id = episode AND slot_id = slot AND released_at IS NULL ORDER BY acquired_at DESC LIMIT 1);
    END IF;
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_account()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
DECLARE r JSONB := to_jsonb(CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END); snap JSONB;
BEGIN
  IF lower(r->>'platform') <> 'openai' OR lower(r->>'type') <> 'oauth' THEN RETURN COALESCE(NEW, OLD); END IF;
  PERFORM oauth_observation_ensure_episode((r->>'id')::BIGINT);
  snap := jsonb_strip_nulls(jsonb_build_object('observation_schema_version','263','source',COALESCE(NULLIF(current_setting('oauth_observation.source',true),''),'unknown'),'rule_id',NULLIF(current_setting('oauth_observation.rule_id',true),''),'quality_outcome',NULLIF(current_setting('oauth_observation.outcome',true),''),'bps_trigger',NULLIF(current_setting('oauth_observation.bps_trigger',true),''),'priority',r->'priority','concurrency',r->'concurrency','load_factor',r->'load_factor','status',r->'status','schedulable',r->'schedulable','expires_at',r->'expires_at','proxy_id',r->'proxy_id','plan_type',r->'credentials'->'plan_type','bps_enabled',r->'credentials'->'bps_enabled','bps_mode',r->'credentials'->'bps_mode','bps_disabled',r->'credentials'->'bps_disabled','usage_quota',r->'credentials'->'usage_quota','quota_remaining',r->'credentials'->'quota_remaining','quota_reset_at',r->'credentials'->'quota_reset_at'));
  IF TG_OP <> 'UPDATE' OR (to_jsonb(OLD) - ARRAY['credentials','extra','error_message']) IS DISTINCT FROM (to_jsonb(NEW) - ARRAY['credentials','extra','error_message']) OR ((to_jsonb(OLD)->'credentials') - ARRAY['access_token','refresh_token','id_token','email']) IS DISTINCT FROM ((to_jsonb(NEW)->'credentials') - ARRAY['access_token','refresh_token','id_token','email']) THEN
    INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot) VALUES ('account',r->>'id',lower(TG_OP),snap);
  END IF;
  RETURN COALESCE(NEW, OLD);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_account_group()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
DECLARE r JSONB := to_jsonb(CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END);
BEGIN
  IF NOT oauth_observation_account_is_eligible((r->>'account_id')::BIGINT) AND NOT EXISTS (SELECT 1 FROM oauth_observation_episodes WHERE account_id=(r->>'account_id')::BIGINT) THEN RETURN COALESCE(NEW,OLD); END IF;
  INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot) VALUES ('account_group',(r->>'account_id')||':'||(r->>'group_id'),lower(TG_OP),jsonb_strip_nulls(jsonb_build_object('source',COALESCE(NULLIF(current_setting('oauth_observation.source',true),''),'unknown'),'rule_id',NULLIF(current_setting('oauth_observation.rule_id',true),''),'quality_outcome',NULLIF(current_setting('oauth_observation.outcome',true),''),'bps_trigger',NULLIF(current_setting('oauth_observation.bps_trigger',true),''),'account_id',r->'account_id','group_id',r->'group_id','priority',r->'priority')));
  RETURN COALESCE(NEW,OLD);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_plan()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
DECLARE r JSONB := to_jsonb(CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END); q JSONB;
BEGIN
  IF NOT oauth_observation_account_is_eligible((r->>'account_id')::BIGINT) AND NOT EXISTS (SELECT 1 FROM oauth_observation_episodes WHERE account_id=(r->>'account_id')::BIGINT) THEN RETURN COALESCE(NEW,OLD); END IF;
  q := r->'pelican_config'->'quality';
  INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot) VALUES ('scheduled_test_plan',r->>'id',lower(TG_OP),jsonb_strip_nulls(jsonb_build_object('source',COALESCE(NULLIF(current_setting('oauth_observation.source',true),''),'unknown'),'rule_id',NULLIF(current_setting('oauth_observation.rule_id',true),''),'quality_outcome',NULLIF(current_setting('oauth_observation.outcome',true),''),'bps_trigger',NULLIF(current_setting('oauth_observation.bps_trigger',true),''),'account_id',r->'account_id','model_id',r->'model_id','cron_expression',r->'cron_expression','enabled',r->'enabled','max_results',r->'max_results','quality',jsonb_strip_nulls(jsonb_build_object('action',q->'action','auto_restore',q->'auto_restore','trigger_on_upstream_5xx',q->'trigger_on_upstream_5xx')))));
  RETURN COALESCE(NEW,OLD);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_result()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
DECLARE r JSONB := to_jsonb(CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END); account BIGINT;
BEGIN
  SELECT account_id INTO account FROM scheduled_test_plans WHERE id=(r->>'plan_id')::BIGINT;
  IF account IS NULL THEN SELECT (snapshot->>'account_id')::BIGINT INTO account FROM oauth_observation_archives WHERE subject_type='scheduled_test_plan' AND subject_key=r->>'plan_id' ORDER BY id DESC LIMIT 1; END IF;
  IF account IS NULL OR (NOT oauth_observation_account_is_eligible(account) AND NOT EXISTS (SELECT 1 FROM oauth_observation_episodes WHERE account_id=account)) THEN RETURN COALESCE(NEW,OLD); END IF;
  INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot) VALUES ('scheduled_test_result',r->>'id',lower(TG_OP),jsonb_strip_nulls(jsonb_build_object('source',COALESCE(NULLIF(current_setting('oauth_observation.source',true),''),'unknown'),'rule_id',NULLIF(current_setting('oauth_observation.rule_id',true),''),'quality_outcome',NULLIF(current_setting('oauth_observation.outcome',true),''),'bps_trigger',NULLIF(current_setting('oauth_observation.bps_trigger',true),''),'plan_id',r->'plan_id','account_id',to_jsonb(account),'status',r->'status','latency_ms',r->'latency_ms','started_at',r->'started_at','finished_at',r->'finished_at','quality_action',r->'quality_action','verdict',r->'quality_judgment'->'verdict')));
  RETURN COALESCE(NEW,OLD);
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_archive_audit_mutation()
RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
DECLARE account BIGINT := NULLIF(NEW.extra->'params'->>'id','')::BIGINT; row JSONB;
BEGIN
  IF NEW.status_code < 200 OR NEW.status_code >= 300 OR NEW.method NOT IN ('POST','PUT','PATCH','DELETE')
     OR NEW.actor_role NOT IN ('admin','super_admin') OR NEW.actor_user_id IS NULL
     OR NEW.action NOT LIKE 'admin.accounts.%' OR account IS NULL THEN RETURN NEW; END IF;
  IF NOT oauth_observation_account_is_eligible(account) AND NOT EXISTS (SELECT 1 FROM oauth_observation_episodes WHERE account_id=account) THEN RETURN NEW; END IF;
  SELECT to_jsonb(a) INTO row FROM accounts a WHERE a.id=account;
  INSERT INTO oauth_observation_archives(subject_type,subject_key,operation,snapshot) VALUES
    ('admin_account_mutation',account::TEXT,'insert',jsonb_strip_nulls(jsonb_build_object(
      'actor_type',NEW.actor_role,'actor_id',NEW.actor_user_id,'action',NEW.action,'reason','admin_api',
      'priority',row->'priority','concurrency',row->'concurrency','load_factor',row->'load_factor','status',row->'status','schedulable',row->'schedulable','expires_at',row->'expires_at','proxy_id',row->'proxy_id',
      'plan_type',row->'credentials'->'plan_type','bps_enabled',row->'credentials'->'bps_enabled','usage_quota',row->'credentials'->'usage_quota','quota_remaining',row->'credentials'->'quota_remaining')));
  RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION oauth_observation_usage_apply(p_row JSONB, p_sign BIGINT)
RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
DECLARE account BIGINT := (p_row->>'account_id')::BIGINT; minute TIMESTAMPTZ := date_trunc('minute',(p_row->>'created_at')::TIMESTAMPTZ); id BIGINT := (p_row->>'id')::BIGINT; old oauth_observation_usage_contributions%ROWTYPE; eligible BOOLEAN;
BEGIN
  SELECT oauth_observation_account_is_eligible(account) OR EXISTS (SELECT 1 FROM oauth_observation_episodes WHERE account_id=account) INTO eligible;
  IF NOT eligible THEN RETURN; END IF;
  IF p_sign < 0 THEN SELECT * INTO old FROM oauth_observation_usage_contributions WHERE usage_log_id=id; IF NOT FOUND THEN RETURN; END IF;
    UPDATE oauth_observation_usage_minutes SET requests=requests-1,input_tokens=input_tokens-old.input_tokens,output_tokens=output_tokens-old.output_tokens,cache_creation_tokens=cache_creation_tokens-old.cache_creation_tokens,cache_read_tokens=cache_read_tokens-old.cache_read_tokens,actual_cost=actual_cost-old.actual_cost,latency_count=latency_count-CASE WHEN old.duration_ms IS NULL THEN 0 ELSE 1 END,latency_ms_total=latency_ms_total-COALESCE(old.duration_ms,0),first_token_count=first_token_count-CASE WHEN old.first_token_ms IS NULL THEN 0 ELSE 1 END,first_token_ms_total=first_token_ms_total-COALESCE(old.first_token_ms,0) WHERE account_id=old.account_id AND minute_at=old.minute_at AND model=old.model AND protocol=old.protocol;
    DELETE FROM oauth_observation_usage_contributions WHERE usage_log_id=id; RETURN;
  END IF;
  INSERT INTO oauth_observation_usage_contributions VALUES (id,account,minute,p_row->>'model','native',COALESCE((p_row->>'input_tokens')::BIGINT,0),COALESCE((p_row->>'output_tokens')::BIGINT,0),COALESCE((p_row->>'cache_creation_tokens')::BIGINT,0),COALESCE((p_row->>'cache_read_tokens')::BIGINT,0),COALESCE((p_row->>'actual_cost')::NUMERIC,0),NULLIF(p_row->>'duration_ms','')::BIGINT,NULLIF(p_row->>'first_token_ms','')::BIGINT);
  INSERT INTO oauth_observation_usage_minutes(account_id,minute_at,model,protocol,requests,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,actual_cost,latency_count,latency_ms_total,first_token_count,first_token_ms_total) VALUES (account,minute,p_row->>'model','native',1,COALESCE((p_row->>'input_tokens')::BIGINT,0),COALESCE((p_row->>'output_tokens')::BIGINT,0),COALESCE((p_row->>'cache_creation_tokens')::BIGINT,0),COALESCE((p_row->>'cache_read_tokens')::BIGINT,0),COALESCE((p_row->>'actual_cost')::NUMERIC,0),CASE WHEN p_row->>'duration_ms' IS NULL THEN 0 ELSE 1 END,COALESCE((p_row->>'duration_ms')::BIGINT,0),CASE WHEN p_row->>'first_token_ms' IS NULL THEN 0 ELSE 1 END,COALESCE((p_row->>'first_token_ms')::BIGINT,0)) ON CONFLICT (account_id,minute_at,model,protocol) DO UPDATE SET requests=oauth_observation_usage_minutes.requests+1,input_tokens=oauth_observation_usage_minutes.input_tokens+EXCLUDED.input_tokens,output_tokens=oauth_observation_usage_minutes.output_tokens+EXCLUDED.output_tokens,cache_creation_tokens=oauth_observation_usage_minutes.cache_creation_tokens+EXCLUDED.cache_creation_tokens,cache_read_tokens=oauth_observation_usage_minutes.cache_read_tokens+EXCLUDED.cache_read_tokens,actual_cost=oauth_observation_usage_minutes.actual_cost+EXCLUDED.actual_cost,latency_count=oauth_observation_usage_minutes.latency_count+EXCLUDED.latency_count,latency_ms_total=oauth_observation_usage_minutes.latency_ms_total+EXCLUDED.latency_ms_total,first_token_count=oauth_observation_usage_minutes.first_token_count+EXCLUDED.first_token_count,first_token_ms_total=oauth_observation_usage_minutes.first_token_ms_total+EXCLUDED.first_token_ms_total;
END $$;
CREATE OR REPLACE FUNCTION oauth_observation_usage_trigger() RETURNS TRIGGER LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$ BEGIN IF TG_OP='UPDATE' THEN PERFORM oauth_observation_usage_apply(to_jsonb(OLD),-1); END IF; IF TG_OP <> 'DELETE' THEN PERFORM oauth_observation_usage_apply(to_jsonb(NEW),1); END IF; RETURN COALESCE(NEW,OLD); END $$;

DROP TRIGGER IF EXISTS oauth_observation_accounts_archive ON accounts;
CREATE TRIGGER oauth_observation_accounts_archive AFTER INSERT OR UPDATE OR DELETE ON accounts FOR EACH ROW EXECUTE FUNCTION oauth_observation_archive_account();
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

CREATE OR REPLACE FUNCTION oauth_observation_prune() RETURNS VOID LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, pg_catalog AS $$
BEGIN
  DELETE FROM oauth_observation_events WHERE id IN (SELECT id FROM oauth_observation_events WHERE recorded_at < clock_timestamp()-INTERVAL '90 days' ORDER BY id LIMIT 5000);
  DELETE FROM oauth_observation_slots WHERE id IN (SELECT id FROM oauth_observation_slots WHERE recorded_at < clock_timestamp()-INTERVAL '90 days' ORDER BY id LIMIT 5000);
  DELETE FROM oauth_observation_usage_contributions WHERE usage_log_id IN (SELECT usage_log_id FROM oauth_observation_usage_contributions WHERE minute_at < clock_timestamp()-INTERVAL '180 days' ORDER BY usage_log_id LIMIT 5000);
  DELETE FROM oauth_observation_usage_minutes WHERE ctid IN (SELECT ctid FROM oauth_observation_usage_minutes WHERE minute_at < clock_timestamp()-INTERVAL '180 days' LIMIT 5000);
  DELETE FROM oauth_observation_recorder_health WHERE instance_id IN (SELECT instance_id FROM oauth_observation_recorder_health WHERE observed_at < clock_timestamp()-INTERVAL '90 days' LIMIT 5000);
END $$;

COMMENT ON TABLE oauth_observation_probe_lifetimes IS 'First healthy native turn-state probe through first later degraded probe; inconclusive, expiry, quota and scheduling transitions never terminate it.';
COMMENT ON TABLE oauth_observation_episode_lifetimes IS 'Primary account episode lifetime across all native probe model/version series; detailed series remain in oauth_observation_probe_lifetimes.';
COMMENT ON TABLE oauth_observation_archives IS 'Whitelisted immutable operational snapshots; deliberately has no foreign keys or raw response/error text.';
