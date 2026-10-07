import importlib.util
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('homepage_release', ROOT / 'ops/release-test-station-homepage.py')
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class HomepageReleaseTests(unittest.TestCase):
    def test_routes_only_new_homepage_files_and_preserves_old_assets_and_api(self):
        config = ':80 {\n  encode zstd gzip\n  @homepage_index path / /home /home/\n  handle @homepage_index { file_server }\n  reverse_proxy /api/* test-station-api:8080\n}\n'
        updated = release.homepage_route(config, 'homepage-abcdef', ['home-assets/new.js', 'home-assets/new.css'])
        self.assertIn('@homepage_brand_release path / /home /home/ /home-assets/new.js /home-assets/new.css', updated)
        self.assertLess(updated.index('handle @homepage_brand_release'), updated.index('handle @homepage_index'))
        self.assertIn('reverse_proxy homepage-abcdef:80', updated)
        self.assertIn('reverse_proxy /api/* test-station-api:8080', updated)
        self.assertIn('handle @homepage_index { file_server }', updated)

    def test_rejects_paths_that_can_change_caddy_syntax(self):
        for path in ['../secret', 'home-assets/a\n}', 'docs/index.html', 'home-assets/a b.js']:
            with self.subTest(path=path), self.assertRaises(ValueError):
                release.homepage_route(':80 {\n}', 'homepage-abcdef', [path])

    def test_next_release_replaces_only_its_previous_route(self):
        config = ':80 {\n  @homepage_index path / /home /home/\n  reverse_proxy /api/* test-station-api:8080\n}\n'
        first = release.homepage_route(config, 'homepage-abcdef', ['home-assets/one.js'])
        second = release.homepage_route(first, 'homepage-fedcba', ['home-assets/two.js'])
        self.assertEqual(second.count('handle @homepage_brand_release'), 1)
        self.assertIn('homepage-abcdef:80', second)
        self.assertIn('/home-assets/one.js', second)
        self.assertIn('homepage-fedcba:80', second)

    def test_live_favicon_probe_rejects_wrong_icon(self):
        release.verify_favicon('<link rel="icon" href="data:image/png;base64,AA==" type="image/png">', 'data:image/png;base64,AA==')
        with self.assertRaises(ValueError):
            release.verify_favicon('<link rel="icon" href="/old.png">', 'data:image/png;base64,AA==')

    def test_public_verification_failure_restores_the_old_route_and_keeps_api_state(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = root / 'releases' / 'old' / 'Caddyfile'
            config.parent.mkdir(parents=True)
            config.write_text(':80 {\n  @homepage_index path / /home /home/\n}\n')
            old_config = config.read_text()
            state = {'source_commit': 'a' * 40, 'image_id': 'api-image'}
            (root / 'release-state.json').write_text(json.dumps(state))
            bundle = root / 'bundle'
            bundle.mkdir()
            (bundle / 'image.tar').write_bytes(b'image')
            index = '<html>homepage</html>'
            manifest = {'source_commit': 'b' * 40, 'source_tree': 'c' * 40, 'previous_commit': 'a' * 40,
                        'image_tag': release.PROJECT + '-homepage:' + 'b' * 40, 'image_id': 'homepage-image',
                        'image_archive_sha256': hashlib.sha256(b'image').hexdigest(),
                        'checksums': {'index.html': hashlib.sha256(index.encode()).hexdigest(), 'home-assets/new.js': 'd' * 64}}
            (bundle / 'manifest.json').write_text(json.dumps(manifest))

            def command(args, data=None):
                if args[:3] == ['docker', 'ps', '-q']:
                    return 'edge'
                if args[-2:] == ['cat', '/etc/caddy/Caddyfile']:
                    return old_config
                if 'adapt' in args or args[-1] == 'http://127.0.0.1:2019/config/':
                    return '{}'
                if args[:3] == ['docker', 'image', 'inspect']:
                    return 'homepage-image'
                if args[-3:] == ['ps', '-q', 'homepage-' + 'b' * 12]:
                    return 'candidate'
                if args[-1] == 'http://127.0.0.1/':
                    return index
                return ''

            with patch.object(release, 'ROOT', root), patch.object(release, 'run', side_effect=command), \
                 patch.object(release, 'inspect', return_value={'Mounts': [{'Destination': '/etc/caddy/Caddyfile', 'Source': str(config)}]}), \
                 patch.object(release, 'reload_caddy') as reloads, \
                 patch.object(release, 'check_public', side_effect=ValueError('public checksum failed')):
                with self.assertRaisesRegex(ValueError, 'public checksum'):
                    release.deploy(bundle)
            self.assertEqual(reloads.call_args_list[-1].args, ('edge', old_config))
            self.assertEqual(config.read_text(), old_config)
            self.assertEqual(json.loads((root / 'release-state.json').read_text()), state)
            record = json.loads((root / 'homepage-releases' / ('b' * 40) / 'deployment.json').read_text())
            self.assertTrue(record['rolled_back'])
            self.assertEqual(record['result'], 'failed')


if __name__ == '__main__':
    unittest.main()
