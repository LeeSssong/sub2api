# Source provenance

Derived from https://github.com/Regert888/gpt-outlook-register at 0c28b4011daf352053149c1b28efdb037975bca2 (remote HEAD checked 2026-09-27).
AGPL-3.0 license retained in LICENSE. ProtocolHelpers extracted from auth_flow.py; fingerprint and Sentinel runtime retained. login.py is a dedicated password/TOTP + Codex PKCE state machine, with no registration, email/SMS verification, password guessing, MFA enrollment or webpage-session token retrieval. No local registration database/runtime/credentials copied.
Source for this deployed version is distributed with the release and exposed via authenticated executor /source.tar.gz and administrator source link.

Build integration: the archive includes build/Dockerfile and build/docker-entrypoint.sh from this release. Install Python dependencies from requirements.txt in a virtual environment; run supervise.py followed by the Sub2API command. The executor binds only 127.0.0.1:8791. To run separately, set TOKEN_GUARD_RELOGIN_KEY and start server.py. Run tests with python -m unittest discover -s tests -v.

Validation: the tested existing account passed password and TOTP verification; subsequent Codex authorization requested extra verification, which this executor rejects. Upstream may require extra verification even with correct credentials. No successful live token issuance is claimed.
