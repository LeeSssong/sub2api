"""Verify actual admitted load and separate account contexts, without Prism traffic."""
import asyncio
import json
import tempfile
import unittest
from types import SimpleNamespace
from unittest import mock

from test_server import adapter
from multiplex_runtime import Admission, TurnJournal
from multiplex_browser import MultiplexBrowser, AccountBrowser


class PoolMetricsTests(unittest.IsolatedAsyncioTestCase):
    async def test_snapshot_separates_executing_and_waiting_requests_per_account(self):
        admission = Admission(adapter, active=3, per_account=1, queued=2)
        one = admission.enter('300', 'thread-a')
        two = admission.enter('301', 'thread-b')
        await one.__aenter__()
        await two.__aenter__()
        waiting = asyncio.create_task(admission.enter('300', 'thread-c').__aenter__())
        await asyncio.sleep(0)
        try:
            stats = admission.snapshot('300')
            self.assertEqual(stats['active'], 2)
            self.assertEqual(stats['queued'], 1)
            self.assertEqual(stats['account_active'], 1)
            self.assertEqual(stats['account_queued'], 1)
            self.assertEqual(stats['peak_active'], 2)
            self.assertEqual(admission.snapshot('301')['account_queued'], 0)
        finally:
            waiting.cancel()
            await asyncio.gather(waiting, return_exceptions=True)
            await two.__aexit__(None, None, None)
            await one.__aexit__(None, None, None)
        self.assertEqual(admission.snapshot('300')['active'], 0)
        self.assertEqual(admission.snapshot('300')['queued'], 0)

    async def test_two_live_accounts_reuse_contexts_and_rotation_cannot_interrupt_them(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = MultiplexBrowser(adapter.State(directory), 'fixture', adapter, accounts=2)
            engine._browser = mock.AsyncMock(return_value=object())
            with mock.patch.object(AccountBrowser, 'open', mock.AsyncMock()):
                first = await engine.account('300', 'first-secret')
                second = await engine.account('301', 'second-secret')
                self.assertIsNot(first, second)
                self.assertIs(await engine.account('300', 'first-secret'), first)
                self.assertEqual((first.refs, second.refs), (2, 1))
                with self.assertRaises(adapter.AdapterError) as failure:
                    await engine.account('300', 'rotated-secret')
                self.assertEqual(failure.exception.code, 'credential_rotation')
                self.assertIs(engine.actors['300'], first)
                with self.assertRaises(adapter.AdapterError) as full:
                    await engine.account('302', 'third-secret')
                self.assertEqual(full.exception.code, 'prism_busy')
            self.assertEqual(len(engine.actors), 2)

    async def test_stage_metrics_include_elapsed_and_account_load_without_credentials(self):
        with tempfile.TemporaryDirectory() as directory:
            engine = MultiplexBrowser(adapter.State(directory), 'fixture', adapter)
            journal = TurnJournal(engine.state, adapter, '300', 'private-session')
            async with engine.admission.enter('300', 'private-session'):
                with self.assertLogs('prism.lifecycle') as captured:
                    engine.observe('prism_prepare_start', journal, model='gpt-6.1-sol', effort='xhigh')
            event = json.loads(captured.records[0].getMessage())
            self.assertEqual(event['account_active'], 1)
            self.assertEqual(event['account_id'], '300')
            self.assertGreaterEqual(event['elapsed_ms'], 0)
            self.assertNotIn('private-session', captured.records[0].getMessage())

    async def test_account_context_configuration_rejects_unbounded_values(self):
        from multiplex_browser import multiplex_settings
        with mock.patch.dict('os.environ', {'PRISM_ADAPTER_MAX_ACCOUNTS':'2'}, clear=True):
            self.assertEqual(multiplex_settings()['accounts'], 2)
        for value in ('0', '3', 'not-a-number'):
            with mock.patch.dict('os.environ', {'PRISM_ADAPTER_MAX_ACCOUNTS':value}, clear=True):
                with self.assertRaises(ValueError):
                    multiplex_settings()
