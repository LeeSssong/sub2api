"""Check backend-owned Prism authentication before creating projects.

The legacy OpenAI cookie can resolve to a guest. Never infer authentication from
HTTP 200, the presence of a token, or a rendered editor. Only return a boolean
identity summary to Python; session tokens remain inside the browser.
"""
SESSION_PROBE = """async () => {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 10000);
  try {
    const response = await window.fetch('/api/auth/session', {
      credentials: 'include', cache: 'no-store', signal: controller.signal
    });
    if (!response.ok) return {status: response.status};
    const data = await response.json();
    const user = data.user ?? data.session?.user;
    return {status: response.status, tier: data.userTier,
      user: user ? {id: typeof user.id === 'string' && user.id.length > 0,
                    is_anonymous: user.is_anonymous} : null};
  } finally { clearTimeout(timer); }
}"""


def validate_session(value, error):
    if not isinstance(value, dict) or value.get('status') not in (200, 401, 403):
        raise error(503, 'prism_session_unavailable',
                    'Prism session check is unavailable; no model request was submitted', not_submitted=True)
    user = value.get('user')
    if (value['status'] != 200 or not isinstance(user, dict) or not user.get('id')
            or user.get('is_anonymous') is not False or value.get('tier') == 'logged_out'):
        raise error(422, 'prism_login_required',
                    'Prism resolved this OAuth credential to a signed-out or anonymous session; '
                    'an authenticated Prism session is required; no model request was submitted',
                    not_submitted=True)


def require_session(page, error):
    try:
        value = page.evaluate(SESSION_PROBE)
    except Exception:
        raise error(503, 'prism_session_unavailable',
                    'Prism session check is unavailable; no model request was submitted', not_submitted=True) from None
    validate_session(value, error)


async def require_session_async(page, error):
    try:
        value = await page.evaluate(SESSION_PROBE)
    except Exception:
        raise error(503, 'prism_session_unavailable',
                    'Prism session check is unavailable; no model request was submitted', not_submitted=True) from None
    validate_session(value, error)
