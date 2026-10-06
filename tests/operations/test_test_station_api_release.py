import importlib.util
import unittest
import contextlib
import io
import json
import tempfile
from unittest.mock import patch
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location('release', ROOT / 'ops/deploy-sub2api-test-station-api.py')
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


class APIReleaseTests(unittest.TestCase):
    def test_clones_only_api_and_keeps_dependencies_and_old_service(self):
        old = {'name': 'sub2api-test-station', 'services': {
            'test-station-api': {'image': 'old', 'environment': {'SERVER_PROCESS_ROLE': 'api', 'SECRET': 'hidden'},
                'volumes': ['app:/app/data'], 'networks': ['test-station'], 'depends_on': {'worker': {}}},
            'worker': {'image': 'old'}}, 'networks': {'test-station': {'name': 'sub2api-test-station-network'}}}
        new, service = release.candidate_config(old, 'test-station-api', 'new', 'a' * 40)
        self.assertEqual(service, 'test-station-api-green')
        self.assertEqual(new['services']['worker'], old['services']['worker'])
        self.assertEqual(new['services']['test-station-api'], old['services']['test-station-api'])
        api = new['services'][service]
        self.assertEqual(api['volumes'], ['app:/app/data'])
        self.assertEqual(api['environment']['SERVER_PROCESS_ROLE'], 'api')
        self.assertEqual(api['environment']['SUB2API_DEPLOYMENT_COMMIT'], 'a' * 40)
        self.assertNotIn('depends_on', api)
        self.assertEqual(old['services']['test-station-api']['image'], 'old')

    def test_next_slot_and_preserved_network_aliases(self):
        old = {'services': {'test-station-api-green': {'environment': {'SERVER_PROCESS_ROLE': 'api'},
            'networks': {'test-station': {'aliases': ['old-alias']}}}}}
        new, service = release.candidate_config(old, 'test-station-api-green', 'new', 'a'*40)
        self.assertEqual(service, 'test-station-api-blue')
        self.assertNotIn('old-alias', new['services'][service]['networks']['test-station']['aliases'])
        self.assertEqual(old['services']['test-station-api-green']['networks']['test-station']['aliases'], ['old-alias'])

    def test_refuses_worker_role(self):
        with self.assertRaises(ValueError):
            release.candidate_config({'services': {'api': {'environment': {'SERVER_PROCESS_ROLE': 'worker'}}}}, 'api', 'new', 'a'*40)

    def test_changes_only_exact_caddy_upstream(self):
        source = ':80 {\n reverse_proxy /api/* test-station-api:8080\n # test-station-api:8080 unchanged comment\n reverse_proxy test-station-api:8080\n}\n'
        new = release.change_upstream(source, 'test-station-api', 'test-station-api-green')
        self.assertEqual(new.count('test-station-api-green:8080'), 2)
        self.assertIn('# test-station-api:8080 unchanged comment', new)
        with self.assertRaises(ValueError):
            release.change_upstream(source, 'missing', 'new')

    def test_counts_only_established_server_connections(self):
        rows = ' 0: 00000000:1F90 0100007F:C123 01\n 1: 00000000:1F90 00000000:0000 0A\n 2: 0100007F:C222 0100007F:1F90 01\n'
        self.assertEqual(release.established_connections(rows), 1)


class HostFlowTests(unittest.TestCase):
    def exercise(self, fail_probe=False):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'station'
            old_release = root / 'releases' / ('b' * 40)
            old_release.mkdir(parents=True)
            old_config = old_release / 'compose.yaml'
            old_config.write_text('{}')
            env = old_release / '.env'
            env.write_text('SECRET=never-print\n')
            env.chmod(0o600)
            caddy = old_release / 'Caddyfile'
            caddy.write_text(':80 {\n reverse_proxy test-station-api:8080\n}\n')
            previous = {'source_commit': 'b' * 40, 'source_tree': 'c' * 40, 'image_id': 'base', 'image_tag': 'old', 'release_dir': str(old_release)}
            (root / 'release-state.json').write_text(json.dumps(previous))
            bundle = Path(tmp) / 'bundle'
            bundle.mkdir()
            (bundle / 'sub2api').write_bytes(b'binary')
            (bundle / 'manifest.json').write_text(json.dumps({'source_commit': 'a' * 40, 'source_tree': 'd' * 40,
                'previous_commit': 'b' * 40, 'base_image_id': 'base', 'binary_sha256': release.hashlib.sha256(b'binary').hexdigest()}))
            config = {'services': {'test-station-api': {'image': 'old', 'environment': {'SERVER_PROCESS_ROLE': 'api'}, 'networks': ['test-station']},
                'test-station-worker': {'image': 'old'}, 'test-station-detector': {'image': 'old'}},
                'networks': {'test-station': {'name': 'sub2api-test-station-network'}}}
            old = {'Image': 'base', 'Config': {'Labels': {'com.docker.compose.service': 'test-station-api', 'com.docker.compose.project.config_files': str(old_config)}}}
            def invoke(args, data=None):
                if args[:3] == ['docker', 'ps', '-q']:
                    return 'caddy' if 'label=com.docker.compose.service=test-station-caddy' in args else 'old-api'
                if args[-3:] == ['config', '--format', 'json']:
                    return json.dumps(config)
                if args[:3] == ['docker', 'image', 'inspect']:
                    return 'base' if args[-1] == 'old' else 'new-image'
                if args[-3:] == ['cat', '/etc/caddy/Caddyfile'] or args[-2:] == ['cat', '/etc/caddy/Caddyfile']:
                    return caddy.read_text()
                if args[-3:] == ['ps', '-q', 'test-station-api-green']:
                    return 'new-api'
                if '/proc/net/tcp' in args:
                    return ''
                return ''
            probes = [RuntimeError('offline'), None] if fail_probe else [None]
            with patch.object(release, 'ROOT', root), patch.object(release, 'run', side_effect=invoke) as commands, \
                 patch.object(release, 'inspect', side_effect=lambda value: old if value == 'old-api' else {'Mounts': [{'Destination': '/etc/caddy/Caddyfile', 'Source': str(caddy)}]}), \
                 patch.object(release, 'healthy'), patch.object(release, 'reload_caddy') as reloads, \
                 patch.object(release, 'public_probes', side_effect=probes), contextlib.redirect_stdout(io.StringIO()):
                if fail_probe:
                    with self.assertRaises(RuntimeError):
                        release.deploy(bundle)
                    self.assertEqual(json.loads((root / 'release-state.json').read_text()), previous)
                    self.assertEqual(reloads.call_args[0][1], ':80 {\n reverse_proxy test-station-api:8080\n}\n')
                else:
                    release.deploy(bundle)
                    before = list(commands.call_args_list)
                    self.assertFalse(any('stop' in call.args[0] for call in before))
                    release.finalize(root / 'releases' / ('a' * 40))
                    self.assertIn(unittest.mock.call(['docker', 'stop', '--time', '10', 'old-api']), commands.call_args_list)
                    self.assertEqual(json.loads((root / 'release-state.json').read_text())['active_api_container'], 'new-api')
                starts = [call.args[0] for call in commands.call_args_list if 'up' in call.args[0]]
                self.assertEqual(len(starts), 1)
                self.assertEqual(starts[0][-4:], ['up', '-d', '--no-deps', 'test-station-api-green'])

    def test_ready_candidate_then_route_then_drain_without_restarting_jobs(self):
        self.exercise()

    def test_failed_public_probe_restores_old_route_and_state(self):
        self.exercise(fail_probe=True)


if __name__ == '__main__':
    unittest.main()
