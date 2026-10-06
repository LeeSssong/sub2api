import copy
import importlib
import json
import sys
import unittest
from datetime import datetime, timezone, timedelta
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / 'plugins/astrbot_plugin_xingqiao_ops'))

def fixture():
    return json.loads((ROOT / 'tests/fixtures/pelican-report-v2.json').read_text())['report']

class ReportSourceV2Tests(unittest.TestCase):
    def setUp(self):
        self.source = importlib.import_module('pelican_report_source')
        self.payload = fixture()
        self.now = datetime(2026, 9, 26, 7, 5, tzinfo=timezone.utc)
        self.calls = []

    def get(self, path):
        self.calls.append(path)
        return copy.deepcopy(self.payload)

    def read(self, **kwargs):
        return self.source.load_detection_report_snapshot(self.get, 6, now=self.now, **kwargs)

    def test_report_is_one_get_and_strips_private_fields(self):
        self.payload['account_id'] = 'private'
        self.payload['group']['credentials'] = 'private'
        self.payload['history']['candy'][0]['raw_error'] = 'private'
        result = self.read()
        self.assertEqual(self.calls, ['/api/v1/admin/pelican-reports/groups/6?window=24h'])
        self.assertEqual(result['schema_version'], 2)
        self.assertEqual(result['statistics']['candy']['success_count'], 1)
        self.assertEqual(result['model_id'], 'gpt-6-astra')
        self.assertEqual(result['rate_multiplier'], 5.0)
        self.assertEqual(result['latest']['source_result_id'], 102)
        self.assertEqual(result['artwork_status'], 'available')
        self.assertNotIn('private', str(result))

    def test_group_and_requested_model_must_match(self):
        self.payload['group']['id'] = 7
        with self.assertRaises(self.source.PelicanSourceUnavailable): self.read()
        self.payload = fixture()
        with self.assertRaises(self.source.PelicanSourceUnavailable): self.read(model_id='other-model')

    def test_window_and_asof_must_describe_the_same_24_hours(self):
        for value in ['2026-09-26T06:05:00Z', 'not-a-time']:
            self.payload['window']['to'] = value
            with self.assertRaises(self.source.PelicanSourceUnavailable): self.read()

    def test_fresh_fetch_does_not_refresh_stale_backend_data(self):
        self.now += timedelta(minutes=16)
        self.assertEqual(self.read()['stats_status'], 'stale')

    def test_stats_must_match_real_history_not_just_valid_ratios(self):
        self.payload['statistics']['candy'].update(success_count=2,total_count=2,observed_count=2)
        with self.assertRaises(self.source.PelicanSourceUnavailable): self.read()

    def test_average_uses_the_same_timed_history(self):
        self.payload['statistics']['pelican']['avg_latency_ms'] = 1000
        with self.assertRaises(self.source.PelicanSourceUnavailable): self.read()

    def test_missing_timing_is_not_zero_latency(self):
        for item in [self.payload['history']['candy'][0], self.payload['latest']['candy']['results'][0]]:
            item.update(started_at=None, completed_at=None, latency_ms=None)
        self.payload['latest']['candy'].update(started_at=None,completed_at=None,latency_ms=None)
        self.payload['statistics']['candy'].update(timed_count=0,avg_latency_ms=None)
        self.assertIsNone(self.read()['statistics']['candy']['avg_latency_ms'])

    def test_ungraded_candy_cannot_be_presented_as_passed(self):
        self.payload['history']['candy'][0]['judgment'] = 'ungraded_candy'
        with self.assertRaises(self.source.PelicanSourceUnavailable): self.read()

    def test_a_completed_failed_execution_cannot_reuse_an_artwork(self):
        self.payload['latest']['pelican']['status'] = 'failed'
        self.payload['latest']['pelican']['results'][0]['status'] = 'failed'
        self.payload['history']['pelican'][0]['status'] = 'failed'
        self.payload['statistics']['pelican'].update(success_count=0,failure_count=1,success_rate=0)
        with self.assertRaises(self.source.PelicanSourceUnavailable): self.read()
        self.payload['artwork'] = None
        self.assertEqual(self.read()['artwork_status'], 'none')

    def test_incomplete_latest_execution_is_pending_and_has_no_duration(self):
        self.payload['latest']['pelican']['expected_count'] = 2
        self.payload['latest']['pelican']['status'] = 'pending'
        self.payload['latest']['pelican']['completed_at'] = None
        self.payload['latest']['pelican']['latency_ms'] = None
        self.payload['artwork'] = None
        result = self.read()
        self.assertEqual(result['executions']['pelican']['status'], 'pending')
        self.assertIsNone(result['executions']['pelican']['completed_at'])
        self.assertIsNone(result['executions']['pelican']['latency_ms'])

    def test_artwork_cannot_come_from_a_different_execution_or_model(self):
        for field, value in [('execution_id','old-run'),('source_result_id',999),('model_id','other-model')]:
            self.payload = fixture()
            self.payload['artwork'][field] = value
            with self.assertRaises(self.source.PelicanSourceUnavailable): self.read()

    def test_close_timestamps_do_not_make_a_shared_round(self):
        self.assertIsNone(self.read()['current_round'])
        self.payload['current_round'] = {
            'round_id':'fabricated','scheduled_for':'2026-09-26T07:00:00Z',
            'started_at':'2026-09-26T07:00:01Z','completed_at':'2026-09-26T07:00:21Z',
            'latency_ms':20000,'candy_execution_id':self.payload['latest']['candy']['execution_id'],
            'pelican_execution_id':self.payload['latest']['pelican']['execution_id']}
        with self.assertRaises(self.source.PelicanSourceUnavailable): self.read()

    def test_real_empty_statistics_keep_null_rates(self):
        self.payload.update(model_id=None,current_round=None,artwork=None)
        for kind in ('candy','pelican'):
            self.payload['history'][kind] = []
            self.payload['latest'][kind] = None
            self.payload['history_meta'][kind].update(total_count=0, returned_count=0)
            self.payload['statistics'][kind] = dict(success_count=0,total_count=0,failure_count=0,
                ungraded_count=0,observed_count=0,success_rate=None,timed_count=0,avg_latency_ms=None,execution_count=0)
        result = self.read()
        self.assertEqual(result['statistics']['candy']['total_count'], 0)
        self.assertIsNone(result['statistics']['candy']['success_rate'])
        self.assertIsNone(result['model_id'])

    def test_bounded_recent_history_preserves_full_sample_statistics(self):
        rows = self.payload['history']['pelican']
        original = rows[0]
        self.payload['history']['pelican'] = [dict(original, result_id=i + 1000,
            execution_id='retained-' + str(i)) for i in range(240)]
        self.payload['history_meta']['pelican'] = dict(total_count=12000, returned_count=240, limit=240)
        self.payload['statistics']['pelican'].update(success_count=12000, total_count=12000,
            observed_count=12000, timed_count=12000, execution_count=12000)
        result = self.read()
        self.assertEqual(result['statistics']['pelican']['total_count'], 12000)
        self.assertEqual(len(result['history']['pelican']), 240)
        self.assertEqual(result['history_meta']['pelican']['total_count'], 12000)
        self.assertEqual(result['latest']['source_result_id'], 102)

    def test_bounded_history_metadata_cannot_conceal_missing_or_false_counts(self):
        for change in ('missing_row', 'wrong_total', 'wrong_limit', 'false_summary'):
            self.payload = fixture()
            if change == 'missing_row':
                self.payload['history']['pelican'] = []
            elif change == 'wrong_total':
                self.payload['history_meta']['pelican']['total_count'] = 100
            elif change == 'wrong_limit':
                self.payload['history_meta']['pelican']['limit'] = 0
            else:
                self.payload['statistics']['pelican']['failure_count'] = 1
            with self.subTest(change=change), self.assertRaises(self.source.PelicanSourceUnavailable):
                self.read()

    def test_source_failure_never_falls_back_to_gallery_or_detector(self):
        def unavailable(path):
            self.calls.append(path)
            raise RuntimeError('private transport failure')
        with self.assertRaises(self.source.PelicanSourceUnavailable) as caught:
            self.source.load_detection_report_snapshot(unavailable, 6, now=self.now)
        self.assertEqual(len(self.calls), 1)
        self.assertNotIn('private', str(caught.exception))

    def test_only_canonical_readonly_report_paths_are_allowed(self):
        from pelican_source import is_allowed_source_path
        self.assertTrue(is_allowed_source_path('/api/v1/admin/pelican-reports/groups/6?window=24h'))
        self.assertTrue(is_allowed_source_path('/api/v1/admin/pelican-reports/groups/6?window=24h&model_id=gpt-6-astra'))
        for path in ['/api/v1/admin/pelican-reports/groups/0',
                     '/api/v1/admin/pelican-reports/groups/6/run',
                     '/api/v1/admin/pelican-reports/groups/6?window=48h',
                     '/api/v1/admin/pelican-reports/groups/6?force=true',
                     '/api/v1/admin/pelican-reports/groups/6?model_id=a&model_id=b',
                     '//evil.test/api/v1/admin/pelican-reports/groups/6']:
            self.assertFalse(is_allowed_source_path(path), path)

    def test_model_filter_matches_backend_100_byte_limit(self):
        self.assertIn('model_id=', self.source.report_path(6, 'a' * 100))
        for model in ('a' * 101, '鹈' * 34):
            with self.assertRaises(ValueError):
                self.source.report_path(6, model)
            from urllib.parse import urlencode
            self.assertFalse(self.source.is_allowed_report_path(
                '/api/v1/admin/pelican-reports/groups/6?' + urlencode({'model_id': model})))
