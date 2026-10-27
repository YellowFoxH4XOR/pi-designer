import base64
import importlib.util
import os
import plistlib
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/release-metadata.py'
spec = importlib.util.spec_from_file_location('release_metadata', SCRIPT)
metadata = importlib.util.module_from_spec(spec)
spec.loader.exec_module(metadata)
KEY = base64.b64encode(bytes(range(32))).decode()
FEED = 'https://github.com/example/releases/releases/latest/download/appcast.xml'


class ReleaseMetadataTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.info = Path(self.temp.name) / 'Info.plist'
        self.info.write_bytes((ROOT / 'mac/Info.plist').read_bytes())

    def configure(self, env):
        metadata.configure_plist(self.info, env)
        return plistlib.loads(self.info.read_bytes())

    def test_local_build_has_no_fake_feed_or_key(self):
        info = self.configure({})
        self.assertNotIn('SUFeedURL', info)
        self.assertNotIn('SUPublicEDKey', info)
        self.assertFalse(info['SUAutomaticallyUpdate'])
        self.assertFalse(info['SUEnableSystemProfiling'])
        self.assertTrue(info['SUVerifyUpdateBeforeExtraction'])
        self.assertTrue(info['SURequireSignedFeed'])

    def test_release_embeds_version_and_public_key_but_no_private_key(self):
        info = self.configure({'PIDESIGNER_RELEASE_BUILD': '1', 'PIDESIGNER_VERSION': '0.3.0', 'SPARKLE_PUBLIC_KEY': KEY, 'PIDESIGNER_UPDATE_FEED_URL': FEED})
        self.assertEqual(info['CFBundleVersion'], '0.3.0')
        self.assertEqual(info['CFBundleShortVersionString'], '0.3.0')
        self.assertEqual(info['SUPublicEDKey'], KEY)
        self.assertEqual(info['SUFeedURL'], FEED)
        self.assertTrue(info['SUEnableAutomaticChecks'])
        self.assertFalse(info['SUAutomaticallyUpdate'])

    def test_missing_or_invalid_configuration_never_modifies_plist(self):
        original = self.info.read_bytes()
        for env in [
            {'PIDESIGNER_RELEASE_BUILD': '1'},
            {'SPARKLE_PUBLIC_KEY': KEY},
            {'PIDESIGNER_UPDATE_FEED_URL': FEED},
            {'SPARKLE_PUBLIC_KEY': 'invalid', 'PIDESIGNER_UPDATE_FEED_URL': FEED},
            {'SPARKLE_PUBLIC_KEY': base64.b64encode(b'short').decode(), 'PIDESIGNER_UPDATE_FEED_URL': FEED},
            {'SPARKLE_PUBLIC_KEY': KEY, 'PIDESIGNER_UPDATE_FEED_URL': 'http://example.com/feed'},
            {'SPARKLE_PUBLIC_KEY': KEY, 'PIDESIGNER_UPDATE_FEED_URL': 'https://user:pass@example.com/feed'},
            {'PIDESIGNER_VERSION': '0.3.0-beta.1'},
        ]:
            with self.subTest(env=env), self.assertRaises(ValueError):
                self.configure(env)
            self.assertEqual(self.info.read_bytes(), original)

    def test_versions_and_feed_paths_cannot_escape_or_inject_workflow_output(self):
        for version in ['v1.0.0', '../1.0.0', '01.2.3', '1.0', '1.0.0\ninjected=true', '1.0.0-beta']:
            with self.subTest(version=version), self.assertRaises(ValueError):
                metadata.version_parts(version)
        for repo in ['../x', 'owner/..', 'https://github.com/o/r', 'owner/repo\ninjected=true']:
            with self.subTest(repo=repo), self.assertRaises(ValueError):
                metadata.feed_url(repo)
        self.assertEqual(metadata.feed_url('example/releases'), FEED)

    def test_release_info_and_monotonic_version_gate(self):
        result = subprocess.run(['python3', str(SCRIPT), 'release-info'], env={**os.environ, 'GITHUB_REF_NAME': 'v1.2.0', 'GITHUB_REPOSITORY': 'private/source', 'RELEASE_REPOSITORY': 'example/releases'}, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('repository=example/releases\n', result.stdout)
        self.assertIn('feed=' + FEED, result.stdout)
        for old, allowed in [('', True), ('v1.1.9', True), ('v1.2.0', False), ('v2.0.0', False)]:
            result = subprocess.run(['python3', str(SCRIPT), 'check-newer', '1.2.0', old], capture_output=True)
            self.assertEqual(result.returncode == 0, allowed)


if __name__ == '__main__':
    unittest.main()
