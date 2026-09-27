import copy
import importlib
import sys
import unittest
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'plugins/astrbot_plugin_xingqiao_ops'))


class PelicanSourceTests(unittest.TestCase):
    def setUp(self):
        try:
            self.source = importlib.import_module('pelican_source')
        except ModuleNotFoundError:
            self.fail('The read-only Pelican source adapter has not been implemented')
        self.now = datetime(2026, 9, 26, 4, tzinfo=timezone.utc)
        self.calls = []
        self.item = {
            'id': 41, 'group_id': 6, 'model_id': 'example-model',
            'reasoning_effort': 'high', 'latency_ms': 12345,
            'generated_at': '2026-09-26T03:30:00Z',
        }
        self.detail = dict(self.item, response_text='<svg><circle r="10"/></svg>',
                           account_name='private-account', account_id=55)
        self.listing = {
            'enabled': True, 'max_items': 20, 'retention_days': 0,
            'stats': {'success_count': 100, 'total_count': 101, 'success_rate': 99.00990099009901},
            'stats_window': {'from': '2026-09-25T04:00:00Z', 'to': '2026-09-26T04:00:00Z',
                             'coverage_started_at': '2026-09-24T04:00:00Z', 'complete': True},
            'groups': [{
                'id': 6, 'name': '公开分组', 'platform': 'openai',
                'stats': {'success_count': 2, 'total_count': 4, 'success_rate': 50},
                'items': [self.item], 'account_ids': [55], 'credentials': 'private',
            }],
        }

    def get(self, path):
        self.calls.append(path)
        if path == '/api/v1/admin/pelican-showcase':
            return copy.deepcopy(self.listing)
        if path == '/api/v1/admin/pelican-showcase/items/41':
            return copy.deepcopy(self.detail)
        self.fail(f'Unexpected source path: {path}')

    def snapshot(self, **kwargs):
        return self.source.load_report_snapshot(self.get, 6, now=self.now, **kwargs)

    def test_selected_group_counts_use_statistics_and_only_public_fields(self):
        result = self.snapshot()
        self.assertEqual(result['stats'], {'success_count': 2, 'total_count': 4, 'success_rate': 50})
        self.assertEqual(result['stats_status'], 'available')
        self.assertEqual(result['artwork_status'], 'available')
        self.assertEqual(result['group_name'], '公开分组')
        self.assertEqual(result['schema_version'], 1)
        self.assertIsInstance(result['fetched_at'], str)
        self.assertEqual(result['latest']['model'], 'example-model')
        self.assertEqual(result['latest']['response_text'], '<svg><circle r="10"/></svg>')
        self.assertEqual(set(result['latest']), {
            'id', 'group_id', 'group_name', 'model', 'reasoning_effort',
            'latency_ms', 'generated_at', 'response_text',
        })
        self.assertNotIn('private', str(result))
        self.assertEqual(self.calls, ['/api/v1/admin/pelican-showcase',
                                      '/api/v1/admin/pelican-showcase/items/41'])

    def test_visible_groups_are_metadata_only_and_fetch_no_artwork(self):
        self.assertEqual(self.source.fetch_visible_groups(self.get), [{'id': 6, 'name': '公开分组'}])
        self.assertEqual(self.calls, ['/api/v1/admin/pelican-showcase'])

    def test_missing_statistics_are_not_zero_or_artwork_count(self):
        for stats in (None, {}, {'success_count': 2, 'total_count': 4, 'success_rate': None}):
            with self.subTest(stats=stats):
                self.listing['groups'][0]['stats'] = stats
                result = self.snapshot()
                self.assertIsNone(result['stats'])
                self.assertEqual(result['stats_status'], 'unavailable')
                self.assertEqual(result['artwork_status'], 'available')

    def test_zero_completed_results_preserve_null_rate(self):
        self.listing['groups'][0]['stats'] = {'success_count': 0, 'total_count': 0, 'success_rate': None}
        result = self.snapshot()
        self.assertEqual(result['stats'], {'success_count': 0, 'total_count': 0, 'success_rate': None})
        self.assertEqual(result['stats_status'], 'available')

    def test_missing_rate_field_is_unavailable_even_when_counts_are_zero(self):
        self.listing['groups'][0]['stats'] = {'success_count': 0, 'total_count': 0}
        result = self.snapshot()
        self.assertIsNone(result['stats'])
        self.assertEqual(result['stats_status'], 'unavailable')

    def test_incomplete_coverage_is_visible(self):
        self.listing['stats_window'].update(coverage_started_at='2026-09-26T02:00:00Z', complete=False)
        result = self.snapshot()
        self.assertEqual(result['stats_status'], 'incomplete')
        self.assertFalse(result['stats_window']['complete'])
        self.assertEqual(result['stats']['total_count'], 4)

    def test_missing_window_keeps_statistics_unavailable(self):
        self.listing.pop('stats_window')
        result = self.snapshot()
        self.assertIsNone(result['stats'])
        self.assertIsNone(result['stats_window'])
        self.assertEqual(result['stats_status'], 'unavailable')

    def test_stats_staleness_and_artwork_staleness_have_separate_thresholds(self):
        self.listing['stats_window'].update(to='2026-09-26T03:44:59Z',
                                            **{'from': '2026-09-25T03:44:59Z'})
        result = self.snapshot()
        self.assertEqual(result['stats_status'], 'stale')
        self.assertEqual(result['artwork_status'], 'available')
        self.assertEqual(self.snapshot(artwork_max_age_seconds=600)['artwork_status'], 'stale')

    def test_old_latest_artwork_is_marked_stale_without_relabelling_it(self):
        self.item['generated_at'] = '2026-09-25T03:59:59Z'
        self.detail['generated_at'] = self.item['generated_at']
        result = self.snapshot()
        self.assertEqual(result['artwork_status'], 'stale')
        self.assertEqual(result['latest']['generated_at'], '2026-09-25T03:59:59+00:00')
        self.assertEqual(result['stats_status'], 'available')

    def test_no_artwork_is_distinct_from_artwork_fetch_failure(self):
        self.listing['groups'][0]['items'] = []
        result = self.snapshot()
        self.assertEqual(result['artwork_status'], 'none')
        self.assertIsNone(result['latest'])
        self.assertEqual(self.calls, ['/api/v1/admin/pelican-showcase'])

    def test_artwork_failure_does_not_discard_valid_statistics_or_error_leak(self):
        def get(path):
            if path.endswith('/items/41'):
                raise RuntimeError('private-url secret-token')
            return self.get(path)
        result = self.source.load_report_snapshot(get, 6, now=self.now)
        self.assertEqual(result['stats_status'], 'available')
        self.assertEqual(result['artwork_status'], 'unavailable')
        self.assertIsNone(result['latest'])
        self.assertNotIn('secret-token', str(result))

    def test_artwork_detail_must_match_the_visible_item_and_group(self):
        for changes in ({'id': 42}, {'group_id': 9}, {'model_id': 'other'},
                        {'generated_at': '2026-09-26T03:29:00Z'}, {'response_text': ''}):
            with self.subTest(changes=changes):
                original = self.detail.copy()
                self.detail.update(changes)
                result = self.snapshot()
                self.assertIsNone(result['latest'])
                self.assertEqual(result['artwork_status'], 'unavailable')
                self.detail = original

    def test_selects_newest_valid_visible_artwork_and_only_one_detail(self):
        older = dict(self.item, id=40, generated_at='2026-09-26T03:00:00Z')
        self.listing['groups'][0]['items'] = [older, self.item]
        self.assertEqual(self.snapshot()['latest']['id'], 41)
        self.assertEqual(len(self.calls), 2)

    def test_rejects_hidden_disabled_or_invalid_group_without_detail_calls(self):
        for group_id, enabled in ((7, True), (6, False), (True, True), ('6', True), (0, True)):
            with self.subTest(group_id=group_id, enabled=enabled):
                self.listing['enabled'] = enabled
                self.calls.clear()
                with self.assertRaises(self.source.PelicanSourceUnavailable):
                    self.source.load_report_snapshot(self.get, group_id, now=self.now)
                self.assertTrue(all('/items/' not in path for path in self.calls))

    def test_disabled_gallery_lists_no_groups(self):
        self.listing['enabled'] = False
        self.assertEqual(self.source.fetch_visible_groups(self.get), [])

    def test_failed_bridge_is_sanitized_and_does_not_fall_back(self):
        def unavailable(path):
            self.calls.append(path)
            raise RuntimeError('https://private.example secret-key')
        with self.assertRaises(self.source.PelicanSourceUnavailable) as caught:
            self.source.load_report_snapshot(unavailable, 6, now=self.now)
        self.assertNotIn('private', str(caught.exception))
        self.assertNotIn('secret-key', str(caught.exception))
        self.assertEqual(self.calls, ['/api/v1/admin/pelican-showcase'])

    def test_rejects_invalid_statistics_and_future_window(self):
        invalid = [
            {'success_count': True, 'total_count': 4, 'success_rate': 25},
            {'success_count': 5, 'total_count': 4, 'success_rate': 100},
            {'success_count': 2, 'total_count': 4, 'success_rate': float('nan')},
            {'success_count': 2, 'total_count': 4, 'success_rate': 0.5},
            {'success_count': 0, 'total_count': 0, 'success_rate': 0},
        ]
        for stats in invalid:
            with self.subTest(stats=stats):
                self.listing['groups'][0]['stats'] = stats
                self.assertEqual(self.snapshot()['stats_status'], 'unavailable')
        self.listing['stats_window']['to'] = '2026-09-27T04:00:00Z'
        self.assertIsNone(self.snapshot()['stats_window'])

    def test_only_exact_read_paths_are_allowed(self):
        for path in ('/api/v1/admin/pelican-showcase', '/api/v1/admin/pelican-showcase/items/41'):
            self.assertTrue(self.source.is_allowed_source_path(path))
        for path in ('https://example.com/api/v1/admin/pelican-showcase',
                     '/api/v1/pelican-showcase', '/api/v1/admin/pelican-test-results',
                     '/api/v1/admin/pelican-showcase?group_id=6',
                     '/api/v1/admin/pelican-showcase/items/0',
                     '/api/v1/admin/pelican-showcase/items/01',
                     '/api/v1/admin/pelican-showcase/items/41/../../accounts',
                     '/api/v1/admin/pelican-showcase/items/41/trigger', None):
            self.assertFalse(self.source.is_allowed_source_path(path))

    def test_oversized_statistics_number_degrades_without_crashing(self):
        self.listing['groups'][0]['stats']['success_rate'] = 10**1000
        result = self.snapshot()
        self.assertIsNone(result['stats'])
        self.assertEqual(result['stats_status'], 'unavailable')
        self.assertEqual(result['artwork_status'], 'available')

    def test_oversized_freshness_option_fails_before_any_read(self):
        with self.assertRaises(self.source.PelicanSourceUnavailable):
            self.snapshot(max_age_seconds=10**1000)
        self.assertEqual(self.calls, [])

    def test_freshness_boundary_is_inclusive(self):
        self.listing['stats_window'].update(to='2026-09-26T03:45:00Z',
                                            **{'from': '2026-09-25T03:45:00Z'})
        result = self.snapshot(artwork_max_age_seconds=1800)
        self.assertEqual(result['stats_status'], 'available')
        self.assertEqual(result['artwork_status'], 'available')

    def test_future_artwork_and_naive_timestamps_are_unavailable(self):
        for stamp in ('2026-09-26T05:00:00Z', '2026-09-26T03:30:00'):
            with self.subTest(stamp=stamp):
                self.item['generated_at'] = stamp
                self.calls.clear()
                result = self.snapshot()
                self.assertEqual(result['artwork_status'], 'unavailable')
                self.assertIsNone(result['latest'])
                self.assertEqual(self.calls, ['/api/v1/admin/pelican-showcase'])

    def test_rejects_duplicate_visibility_identity(self):
        self.listing['groups'].append(copy.deepcopy(self.listing['groups'][0]))
        with self.assertRaises(self.source.PelicanSourceUnavailable):
            self.snapshot()

    def test_path_id_must_fit_the_backend_signed_integer(self):
        self.assertTrue(self.source.is_allowed_source_path(
            '/api/v1/admin/pelican-showcase/items/9223372036854775807'))
        self.assertFalse(self.source.is_allowed_source_path(
            '/api/v1/admin/pelican-showcase/items/9223372036854775808'))


if __name__ == '__main__':
    unittest.main()
