import sys
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'plugins/astrbot_plugin_xingqiao_ops'))
from performance import summarize, load_snapshot, is_performance_query, render_card


class PerformanceTests(unittest.TestCase):
    def setUp(self):
        self.end = datetime(2026, 9, 21, 14, tzinfo=timezone.utc)
        self.start = self.end - timedelta(hours=1)

    def row(self, **kw):
        return dict(dict(id=1, group_id=6, created_at=self.start.isoformat(),
                         first_token_ms=1000, duration_ms=2000,
                         input_tokens=10, cache_read_tokens=90,
                         cache_creation_tokens=0), **kw)

    def test_window_median_and_weighted_cache(self):
        rows = [self.row(id=i, first_token_ms=i*1000) for i in range(20)]
        rows += [self.row(id=100, created_at=(self.start-timedelta(seconds=1)).isoformat()),
                 self.row(id=101, created_at=self.end.isoformat())]
        result = summarize(rows, self.start, self.end)
        self.assertEqual(result['first'], '9.5 s')
        self.assertEqual(result['cache'], '90.0%')
        self.assertEqual(summarize([], self.start, self.end)['cache'], '暂无数据')
        self.assertEqual(summarize(rows[:1], self.start, self.end)['cache'], '样本不足')

    def test_missing_metrics_not_zero(self):
        result = summarize([self.row(first_token_ms=None, duration_ms=None)], self.start, self.end)
        self.assertEqual(result['first'], '暂无数据')
        self.assertEqual(result['duration'], '暂无数据')

    def test_trigger(self):
        for text in ['性能', '@小星 现在性能怎么样', '查一下性能', '当前性能']:
            self.assertTrue(is_performance_query(text))
        for text in ['怎么优化代码性能', '性能卡片怎么做', '不要查性能', '缓存是什么']:
            self.assertFalse(is_performance_query(text))

    def test_fetch_all_pages_and_only_public_openai_groups(self):
        from urllib.parse import urlparse, parse_qs
        calls = []
        def get(path):
            calls.append(path)
            if '/groups?' in path:
                return {'items': [
                    dict(id=6, name='GPT-Plus', rate_multiplier=.2, platform='openai', status='active', is_exclusive=False, sort_order=1),
                    dict(id=7, name='私有示例', platform='openai', status='active', is_exclusive=True),
                    dict(id=8, name='动态公开分组', rate_multiplier=.1, platform='openai', status='active', is_exclusive=False, sort_order=2),
                    dict(id=9, name='停用示例', platform='openai', status='disabled', is_exclusive=False),
                    dict(id=10, name='其他平台示例', platform='anthropic', status='active', is_exclusive=False),
                    dict(id=11, name='缺少公开属性示例'),
                ], 'pages': 1}
            query = parse_qs(urlparse(path).query)
            page, group_id = int(query['page'][0]), int(query['group_id'][0])
            if group_id == 8:
                return {'items': [], 'pages': 1, 'total': 0}
            return {'items': [self.row(id=page, group_id=group_id)], 'pages': 2, 'total': 2}
        snapshot = load_snapshot(get, self.end)
        self.assertEqual([g['name'] for g in snapshot['groups']], ['GPT-Plus', '动态公开分组'])
        self.assertEqual([(parse_qs(urlparse(p).query)['group_id'][0], parse_qs(urlparse(p).query)['page'][0])
                          for p in calls if '/usage?' in p], [('6', '1'), ('6', '2'), ('8', '1')])
        self.assertEqual(snapshot['groups'][0]['cache'], '样本不足')
        self.assertEqual(snapshot['groups'][1]['first'], '暂无数据')
        image = render_card(snapshot)
        self.assertTrue(image.startswith(b'\x89PNG'))

    def test_fetch_failure_not_empty_success(self):
        with self.assertRaises(ValueError):
            load_snapshot(lambda path: None, self.end)

    def test_stop_paging_after_window(self):
        calls = []
        def get(path):
            calls.append(path)
            if '/groups?' in path:
                return {'items': [dict(id=6, name='GPT-Plus', rate_multiplier=.2,
                                      platform='openai', status='active', is_exclusive=False)], 'pages': 1}
            return {'items': [self.row(created_at=(self.start-timedelta(seconds=1)).isoformat())], 'pages': 100}
        snapshot = load_snapshot(get, self.end)
        self.assertEqual(len(calls), 2)
        self.assertEqual(snapshot['groups'][0]['first'], '暂无数据')


if __name__ == '__main__':
    unittest.main()
