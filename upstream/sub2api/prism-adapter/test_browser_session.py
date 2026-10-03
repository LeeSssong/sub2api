"""Session admission regressions: authenticated HTTP 200 is not anonymous HTTP 200."""
import unittest
from unittest import mock
from test_server import adapter
import browser_session


class SessionTests(unittest.TestCase):
    def test_guest_session_cannot_create_project_or_submit_model(self):
        for value in [
            {'status': 200, 'user': {'id': 'guest', 'is_anonymous': True}},
            {'status': 200, 'user': {'id': 'guest', 'is_anonymous': False}, 'tier': 'logged_out'},
            {'status': 200, 'user': None},
            {'status': 401},
        ]:
            with self.subTest(value=value):
                page = mock.Mock()
                page.evaluate.return_value = value
                with self.assertRaises(adapter.AdapterError) as error:
                    browser_session.require_session(page, adapter.AdapterError)
                self.assertEqual(error.exception.code, 'prism_login_required')
                self.assertTrue(error.exception.not_submitted)

    def test_session_without_explicit_authenticated_identity_is_rejected(self):
        page = mock.Mock()
        page.evaluate.return_value = {'status': 200, 'user': {'id': 'unknown'}}
        with self.assertRaises(adapter.AdapterError) as error:
            browser_session.require_session(page, adapter.AdapterError)
        self.assertEqual(error.exception.code, 'prism_login_required')

    def test_upstream_outage_is_distinct_from_signed_out(self):
        page = mock.Mock()
        page.evaluate.return_value = {'status': 503}
        with self.assertRaises(adapter.AdapterError) as error:
            browser_session.require_session(page, adapter.AdapterError)
        self.assertEqual(error.exception.code, 'prism_session_unavailable')

    def test_verified_member_session_is_accepted(self):
        page = mock.Mock()
        page.evaluate.return_value = {'status': 200, 'user': {'id': 'member', 'is_anonymous': False}, 'tier': 'paid'}
        browser_session.require_session(page, adapter.AdapterError)

    def test_probe_failure_is_not_misreported_as_login_failure(self):
        page = mock.Mock()
        page.evaluate.side_effect = RuntimeError('private credential fixture')
        with self.assertRaises(adapter.AdapterError) as error:
            browser_session.require_session(page, adapter.AdapterError)
        self.assertEqual(error.exception.code, 'prism_session_unavailable')
        self.assertNotIn('private', str(error.exception))


class AsyncSessionTests(unittest.IsolatedAsyncioTestCase):
    async def test_async_guest_is_rejected_before_project_creation(self):
        page = mock.Mock()
        page.evaluate = mock.AsyncMock(return_value={'status': 200, 'user': {'id': 'guest', 'is_anonymous': True}})
        with self.assertRaises(adapter.AdapterError) as error:
            await browser_session.require_session_async(page, adapter.AdapterError)
        self.assertEqual(error.exception.code, 'prism_login_required')
        self.assertTrue(error.exception.not_submitted)


if __name__ == '__main__':
    unittest.main()
