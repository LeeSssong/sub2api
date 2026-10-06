import copy
import json
import sys
import unittest
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT/'plugins/astrbot_plugin_xingqiao_ops'))

def snapshot():
    from pelican_report_source import load_detection_report_snapshot
    raw = json.loads((ROOT/'tests/fixtures/pelican-report-v2.json').read_text())['report']
    return load_detection_report_snapshot(lambda path:copy.deepcopy(raw), 6,
        now=datetime(2026,9,26,7,5,tzinfo=timezone.utc))

class ReportDeliveryV2Tests(unittest.TestCase):
    def test_candy_changes_invalidate_rendered_report(self):
        from pelican_delivery import content_digest
        before = snapshot()
        after = copy.deepcopy(before)
        after['statistics']['candy']['success_count'] = 0
        self.assertNotEqual(content_digest(before), content_digest(after))

    def test_multiplier_model_and_round_changes_invalidate_rendered_report(self):
        from pelican_delivery import content_digest
        for key, value in [('rate_multiplier',2),('model_id','other-model'),('current_round',{'round_id':'new'})]:
            with self.subTest(key=key):
                before = snapshot()
                after = copy.deepcopy(before)
                after[key] = value
                self.assertNotEqual(content_digest(before),content_digest(after))

    def test_new_read_time_alone_does_not_resend_same_content(self):
        from pelican_delivery import content_digest
        before = snapshot()
        after = copy.deepcopy(before)
        after.update(fetched_at='2026-09-26T07:06:00Z',as_of='2026-09-26T07:06:00Z',snapshot_id='another-clock-only-id')
        after['stats_window'] = {'from':'2026-09-25T07:06:00Z','to':'2026-09-26T07:06:00Z'}
        self.assertEqual(content_digest(before),content_digest(after))

    def test_candy_only_and_all_failed_reports_are_usable(self):
        from pelican_delivery import report_is_usable
        value = snapshot()
        value.update(latest=None,artwork_status='none')
        value['statistics']['pelican']['observed_count'] = 0
        self.assertTrue(report_is_usable(value))
        value['statistics']['candy']['success_count'] = 0
        self.assertTrue(report_is_usable(value))
        value['stats_status'] = 'stale'
        self.assertFalse(report_is_usable(value))

    def test_true_empty_v2_report_is_not_a_sendable_success(self):
        from pelican_delivery import report_is_usable
        value = snapshot()
        value.update(latest=None,artwork_status='none', current={'candy': None, 'pelican': None})
        for stats in value['statistics'].values(): stats['observed_count'] = 0
        self.assertFalse(report_is_usable(value))
