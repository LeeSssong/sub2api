-- Keep legacy pricing readable while the old API drains and if traffic is
-- returned to it. The official 239 migration remains immutable.
CREATE OR REPLACE FUNCTION sync_channel_reasoning_pricing_compat() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.reasoning_effort_multipliers <> '{}'::jsonb THEN
            NEW.max_reasoning_effort_multiplier := (NEW.reasoning_effort_multipliers->>'max')::numeric;
        ELSIF NEW.max_reasoning_effort_multiplier IS NOT NULL THEN
            NEW.reasoning_effort_multipliers := jsonb_build_object('max', NEW.max_reasoning_effort_multiplier);
        END IF;
    ELSIF NEW.reasoning_effort_multipliers IS DISTINCT FROM OLD.reasoning_effort_multipliers THEN
        NEW.max_reasoning_effort_multiplier := (NEW.reasoning_effort_multipliers->>'max')::numeric;
    ELSIF NEW.max_reasoning_effort_multiplier IS DISTINCT FROM OLD.max_reasoning_effort_multiplier THEN
        IF NEW.max_reasoning_effort_multiplier IS NULL THEN
            NEW.reasoning_effort_multipliers := NEW.reasoning_effort_multipliers - 'max';
        ELSE
            NEW.reasoning_effort_multipliers := NEW.reasoning_effort_multipliers ||
                jsonb_build_object('max', NEW.max_reasoning_effort_multiplier);
        END IF;
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS channel_reasoning_pricing_compat ON channel_model_pricing;
CREATE TRIGGER channel_reasoning_pricing_compat
BEFORE INSERT OR UPDATE ON channel_model_pricing
FOR EACH ROW EXECUTE FUNCTION sync_channel_reasoning_pricing_compat();

CREATE OR REPLACE FUNCTION sync_group_reasoning_pricing_compat() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF jsonb_typeof(NEW.model_pricing) <> 'array' THEN
        RETURN NEW;
    END IF;
    SELECT COALESCE(jsonb_agg(
        CASE
            WHEN jsonb_typeof(entry) <> 'object' THEN entry
            WHEN entry ? 'reasoning_effort_multipliers' THEN
                CASE WHEN jsonb_typeof(entry->'reasoning_effort_multipliers'->'max') = 'number'
                    THEN entry || jsonb_build_object('max_reasoning_effort_multiplier',
                        entry->'reasoning_effort_multipliers'->'max')
                    ELSE entry - 'max_reasoning_effort_multiplier'
                END
            WHEN jsonb_typeof(entry->'max_reasoning_effort_multiplier') = 'number' THEN
                entry || jsonb_build_object('reasoning_effort_multipliers',
                    jsonb_build_object('max', entry->'max_reasoning_effort_multiplier'))
            ELSE entry
        END ORDER BY ordinal
    ), '[]'::jsonb) INTO NEW.model_pricing
    FROM jsonb_array_elements(NEW.model_pricing) WITH ORDINALITY AS pricing(entry, ordinal);
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS group_reasoning_pricing_compat ON groups;
CREATE TRIGGER group_reasoning_pricing_compat
BEFORE INSERT OR UPDATE OF model_pricing ON groups
FOR EACH ROW EXECUTE FUNCTION sync_group_reasoning_pricing_compat();

-- 239 consumed the legacy group key. Restore it from the new map for
-- already configured groups; no map means no legacy multiplier.
UPDATE groups SET model_pricing = model_pricing
WHERE jsonb_typeof(model_pricing) = 'array'
  AND model_pricing @? '$[*].reasoning_effort_multipliers.max';
