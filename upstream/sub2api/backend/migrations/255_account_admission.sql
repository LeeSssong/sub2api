CREATE TABLE account_admission_jobs (
 account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
 target_group_ids BIGINT[] NOT NULL CHECK(cardinality(target_group_ids)>0),
 test_group_id BIGINT,
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','admitted','quarantined','paused')),
 active BOOLEAN NOT NULL DEFAULT true,
 blocked BOOLEAN NOT NULL DEFAULT true,
 generation BIGINT NOT NULL DEFAULT 1,
 next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 lease_token TEXT,
 lease_until TIMESTAMPTZ,
 credentials_signature TEXT,
 groups_snapshot JSONB,
 schedulable_snapshot BOOLEAN,
 last_outcome TEXT,
 candy_result JSONB,
 pelican_result JSONB,
 last_run_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX account_admission_due_idx ON account_admission_jobs(next_run_at) WHERE active;

-- Routine token refresh does not change admission identity. Explicit admin
-- credential edits additionally advance generation using a transaction flag.
CREATE FUNCTION account_admission_credential_signature(c JSONB, p TEXT, t TEXT, proxy BIGINT) RETURNS TEXT
LANGUAGE SQL IMMUTABLE AS $$
 SELECT md5((COALESCE(c,'{}'::jsonb)-ARRAY['access_token','refresh_token','id_token','expires_at','expires_in','token_type','_token_version'])::text || '|' || p || '|' || t || '|' || COALESCE(proxy::text,''));
$$;
CREATE FUNCTION guard_account_admission() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE gate BOOLEAN; worker BOOLEAN := COALESCE(current_setting('sub2api.admission_worker',true),'')='on';
BEGIN
 SELECT blocked INTO gate FROM account_admission_jobs WHERE account_id=NEW.id;
 IF NOT FOUND THEN RETURN NEW; END IF;
 IF NOT worker THEN
  IF (COALESCE(current_setting('sub2api.admission_manual_credentials',true),'')='on' AND OLD.credentials IS DISTINCT FROM NEW.credentials)
    OR account_admission_credential_signature(OLD.credentials,OLD.platform,OLD.type,OLD.proxy_id) IS DISTINCT FROM account_admission_credential_signature(NEW.credentials,NEW.platform,NEW.type,NEW.proxy_id) THEN
   UPDATE account_admission_jobs SET generation=generation+1,lease_token=NULL,lease_until=NULL,next_run_at=NOW(),updated_at=NOW() WHERE account_id=NEW.id;
  END IF;
  IF COALESCE(current_setting('sub2api.admission_manual_scheduling',true),'')='on' OR (OLD.schedulable AND NOT NEW.schedulable) THEN
   -- Only an explicit administrator scheduling action can take over the gate.
   IF NEW.schedulable AND COALESCE(current_setting('sub2api.admission_manual_scheduling',true),'')='on' THEN gate := false; END IF;
   UPDATE account_admission_jobs SET active=false,state='paused',blocked=gate,generation=generation+1,lease_token=NULL,lease_until=NULL,updated_at=NOW() WHERE account_id=NEW.id;
  END IF;
 END IF;
 -- Full Extra replacement and legacy recovery cannot erase the gate.
 NEW.extra := jsonb_set(COALESCE(NEW.extra,'{}'::jsonb),'{account_admission_blocked}',to_jsonb(gate),true);
 IF gate THEN NEW.schedulable := false; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER account_admission_gate BEFORE UPDATE ON accounts FOR EACH ROW EXECUTE FUNCTION guard_account_admission();

CREATE FUNCTION pause_admission_on_membership_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE aid BIGINT := CASE WHEN TG_OP='DELETE' THEN OLD.account_id ELSE NEW.account_id END;
BEGIN
 IF COALESCE(current_setting('sub2api.admission_worker',true),'')<>'on' THEN
  PERFORM id FROM accounts WHERE id=aid FOR UPDATE;
  UPDATE account_admission_jobs SET active=false,state='paused',generation=generation+1,lease_token=NULL,lease_until=NULL,updated_at=NOW() WHERE account_id=aid AND active;
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER account_admission_membership BEFORE INSERT OR UPDATE OR DELETE ON account_groups FOR EACH ROW EXECUTE FUNCTION pause_admission_on_membership_change();
