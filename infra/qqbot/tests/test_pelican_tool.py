import contextlib
import importlib.util
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]


class ReportToolTests(unittest.TestCase):
    def tool(self):
        path = ROOT/'report_tool.py'
        self.assertTrue(path.is_file(), 'Offline report tool is missing')
        spec = importlib.util.spec_from_file_location('report_tool', path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        return module

    def test_missing_config_validation_is_disabled(self):
        tool = self.tool()
        with tempfile.TemporaryDirectory() as tmp, contextlib.redirect_stdout(io.StringIO()) as out:
            self.assertEqual(tool.main(['validate-config', str(Path(tmp)/'missing.json')]), 0)
        state = json.loads(out.getvalue())
        self.assertFalse(state['enabled'])
        self.assertTrue(state['dry_run'])
        self.assertEqual(state['target_count'], 0)

    def test_fixture_renders_public_html_without_network_or_credentials(self):
        tool = self.tool()
        with tempfile.TemporaryDirectory() as tmp:
            destination = Path(tmp)/'report.html'
            with patch('urllib.request.OpenerDirector.open', side_effect=AssertionError('offline test')):
                with contextlib.redirect_stdout(io.StringIO()):
                    result = tool.main(['render', '--fixture', str(ROOT/'tests/fixtures/pelican-public-report.json'),
                                        '--group', '7', '--html-only', '--output', str(destination)])
            self.assertEqual(result, 0)
            html = destination.read_text()
            self.assertIn('【示例数据】离线自测', html)
            self.assertIn('75.0%', html)
            self.assertNotIn('X-API-Key', html)

    def test_hidden_fixture_group_fails_without_an_output(self):
        tool = self.tool()
        with tempfile.TemporaryDirectory() as tmp:
            destination = Path(tmp)/'report.html'
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()) as err:
                result = tool.main(['render', '--fixture', str(ROOT/'tests/fixtures/pelican-public-report.json'),
                                    '--group', '999', '--html-only', '--output', str(destination)])
            self.assertEqual(result, 2)
            self.assertFalse(destination.exists())
            self.assertNotIn('response_text', err.getvalue())

    def test_v2_fixture_is_visibly_marked_as_offline(self):
        tool = self.tool()
        with tempfile.TemporaryDirectory() as tmp, contextlib.redirect_stdout(io.StringIO()):
            destination = Path(tmp)/'report.html'
            result = tool.main(['render', '--fixture', str(ROOT/'tests/fixtures/pelican-report-v2.json'),
                                '--group', '6', '--html-only', '--output', str(destination)])
            self.assertEqual(result, 0)
            self.assertIn('【示例数据】离线自测', destination.read_text())


if __name__ == '__main__':
    unittest.main()
