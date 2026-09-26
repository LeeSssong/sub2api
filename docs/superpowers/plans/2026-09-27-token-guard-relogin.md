# Native token guard implementation plan

Goal: native account probes and password/TOTP-only relogin, mandatory ID-bound credentials, plaintext administrator settings, blue/green production deployment.
Architecture: Go retains scheduler, lease, CAS and ownership recovery; Python login executor packaged in same release image, loopback authenticated endpoint, no account database. Derived login helpers pinned to Regert888/gpt-outlook-register 0c28b4011daf352053149c1b28efdb037975bca2. No schema migration.
Spec: user-approved conversation design with explicit plaintext override and latest-source requirement.

- [ ] Add failing Python protocol tests: password/TOTP success, signup/mail/phone fail closed, callback state mismatch, secret-free errors, target identity mismatch.
- [ ] Extract only necessary login/OAuth helpers and runtime dependencies with license and source provenance; no register/mail/SMS/2FA-enrollment functions. Add bounded loopback executor and entrypoint lifecycle.
- [ ] Go backend native probe, stable account IDs, mandatory three-field credentials, plaintext writes/reads with legacy encrypted reads, CAS and post-write probe, failure backoff; targeted Go tests.
- [ ] Frontend editable credential rows, native mode without endpoints, exact password preservation, available account selection, relevant UI tests/typecheck.
- [ ] Integrate isolated commits, verify Docker runtime and real login against one authorized existing account without exposing secrets; record exact outcomes.
- [ ] Commit and push clean main, execute existing source-verified image/build blue-green chain, candidate readiness then switch, public feature verification, old connection drain <=300 seconds. Preserve rollback artifacts, update worker because guard runs there, preserve detector and shared services.
- [ ] One release record with source identities/digest, tests, stage durations, outcome, rollback, unresolved limitations. Test station not queried/synchronized.
