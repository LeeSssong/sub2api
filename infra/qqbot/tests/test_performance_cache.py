import ast
import asyncio
import os
import sys
import tempfile
import time
import unittest
from pathlib import Path
from types import SimpleNamespace

PLUGIN = Path(__file__).resolve().parents[1] / 'plugins/astrbot_plugin_xingqiao_ops'
sys.path.insert(0, str(PLUGIN))
import performance


class CacheTests(unittest.TestCase):
    def test_atomic_latest_only_and_expiry(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'latest.png'
            performance.write_cached_card(path, b'first', time.time())
            performance.write_cached_card(path, b'second', time.time())
            self.assertEqual(performance.read_cached_card(path), b'second')
            self.assertEqual(list(Path(tmp).iterdir()), [path])
            # Hourly refresh permits the existing 65-minute cache lifetime.
            os.utime(path, (time.time()-3901, time.time()-3901))
            with self.assertRaises(ValueError):
                performance.read_cached_card(path)

    def test_cache_remains_valid_between_hourly_updates(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'latest.png'
            performance.write_cached_card(path, b'image', time.time()-3599)
            self.assertEqual(performance.read_cached_card(path), b'image')

    def test_next_hour_boundary(self):
        self.assertEqual(performance.next_refresh_delay(3600), 3600)
        self.assertEqual(performance.next_refresh_delay(3601), 3599)
        self.assertEqual(performance.next_refresh_delay(7199), 1)

    def test_failed_replace_preserves_old(self):
        from unittest.mock import patch
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'latest.png'
            performance.write_cached_card(path, b'first', time.time())
            with patch.object(performance.os, 'replace', side_effect=OSError):
                with self.assertRaises(OSError):
                    performance.write_cached_card(path, b'second', time.time())
            self.assertEqual(path.read_bytes(), b'first')
            self.assertEqual(list(Path(tmp).iterdir()), [path])

    def test_handler_does_not_stop_before_yield(self):
        # Execute the deployed handler's body without importing the AstrBot runtime.
        tree = ast.parse((PLUGIN / 'main.py').read_text())
        cls = next(n for n in tree.body if isinstance(n, ast.ClassDef) and n.name == 'Main')
        method = next(n for n in cls.body if isinstance(n, ast.AsyncFunctionDef) and n.name == 'performance_card')
        method.decorator_list = []
        scope = dict(AstrMessageEvent=object, asyncio=asyncio,
                     MessageType=SimpleNamespace(GROUP_MESSAGE='group'),
                     is_performance_query=performance.is_performance_query,
                     read_cached_card=lambda path: b'image',
                     message_ats_bot=lambda event: True,
                     Image=SimpleNamespace(fromBytes=lambda data: data),
                     logger=SimpleNamespace(warning=lambda *args: None))
        exec(compile(ast.Module(body=[method], type_ignores=[]), '<handler>', 'exec'), scope)
        class Event:
            is_at_or_wake_command = True
            message_str = '性能'
            stopped = False
            def get_message_type(self): return 'group'
            def stop_event(self): self.stopped = True
            def chain_result(self, chain): return chain
            def plain_result(self, text): return text
        async def run():
            event = Event()
            bot = SimpleNamespace(_performance_path=Path('/unused'), _performance_lock=asyncio.Lock())
            gen = scope['performance_card'](bot, event)
            await gen.__anext__()
            self.assertFalse(event.stopped, 'AstrBot skips send stage when stopped before yield')
            with self.assertRaises(StopAsyncIteration):
                await gen.__anext__()
            self.assertTrue(event.stopped)
        asyncio.run(run())


if __name__ == '__main__':
    unittest.main()
