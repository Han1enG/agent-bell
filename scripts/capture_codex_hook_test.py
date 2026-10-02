import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name('capture-codex-hook.py')
spec = importlib.util.spec_from_file_location('capture', SCRIPT)
capture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(capture)


class CaptureTests(unittest.TestCase):
    def test_redaction_preserves_shape_and_semantics(self):
        payload = {'hook_event_name': 'PermissionRequest', 'permission_mode': 'default',
                   'cwd': '/private/project', 'session_id': 'private-session',
                   'tool_input': {'command': 'secret command', 'nested': [{'token': 'secret'}]},
                   'future_flag': True, 'attempts': 2, 'optional': None}
        result = capture.redact(payload)
        self.assertEqual(result['hook_event_name'], payload['hook_event_name'])
        self.assertEqual(result['permission_mode'], 'default')
        self.assertEqual(result['tool_input']['nested'], [{'token': 'REDACTED'}])
        self.assertTrue(result['future_flag'])
        self.assertEqual(result['attempts'], 2)
        self.assertIsNone(result['optional'])
        self.assertNotIn('secret', json.dumps(result))
        self.assertEqual(set(result), set(payload))

    def test_tap_never_emits_approval_decision(self):
        with tempfile.TemporaryDirectory() as directory:
            result = subprocess.run([sys.executable, str(SCRIPT), directory],
                                    input='{"hook_event_name":"Stop","message":"secret"}',
                                    capture_output=True, text=True, check=True)
            self.assertEqual(result.stdout, '')
            files = list(Path(directory).glob('*.json'))
            self.assertEqual(len(files), 1)
            self.assertEqual(files[0].stat().st_mode & 0o777, 0o600)
            self.assertEqual(json.loads(files[0].read_text())['message'], 'REDACTED')

    def test_malformed_input_is_non_deciding(self):
        with tempfile.TemporaryDirectory() as directory:
            result = subprocess.run([sys.executable, str(SCRIPT), directory], input='invalid',
                                    capture_output=True, text=True, check=True)
            self.assertEqual(result.stdout, '')
            self.assertEqual(list(Path(directory).iterdir()), [])


if __name__ == '__main__':
    unittest.main()
