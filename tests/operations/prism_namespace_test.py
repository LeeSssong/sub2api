import importlib.util
from pathlib import Path
import unittest

root = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('reconcile', root / 'ops/sub2api-prism-reconcile.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class NamespaceTest(unittest.TestCase):
    def test_stopped_container_has_no_adapter(self):
        self.assertIsNone(module.desired_namespace({'State': {'Running': False}}))

    def test_other_project_rejected(self):
        with self.assertRaises(ValueError):
            module.desired_namespace({'State': {'Running': True, 'Pid': 200}, 'Config': {'Labels': {'com.docker.compose.project': 'other'}}})

    def test_live_namespace_tracks_replacement(self):
        c = {'Id': 'old', 'State': {'Running': True, 'Pid': 200}, 'Config': {'Labels': {'com.docker.compose.project': 'sub2api'}}}
        self.assertEqual(module.desired_namespace(c), ('old', '/proc/200/ns/net'))
        c['Id'], c['State']['Pid'] = 'new', 201
        self.assertEqual(module.desired_namespace(c), ('new', '/proc/201/ns/net'))

    def test_host_namespace_rejected(self):
        with self.assertRaises(ValueError):
            module.desired_namespace({'State': {'Running': True, 'Pid': 1}, 'Config': {'Labels': {'com.docker.compose.project': 'sub2api'}}})


if __name__ == '__main__':
    unittest.main()
