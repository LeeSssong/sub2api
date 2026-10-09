package repository

// Shared by the admin raw overview and user route SLA timeline. Count-token
// probes are excluded by both callers before applying this error predicate.
const opsSLAErrorPredicate = `COALESCE(status_code, 0) >= 400 AND NOT is_business_limited AND error_type <> 'client_canceled'`
