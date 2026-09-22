"""Offline authentication boundary checks; never contacts GitHub or pushes images."""
import base64
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
SETUP = ROOT / 'scripts/ci/configure-private-modules.sh'
TOKEN = 'isolated-token-not-a-credential'


class PrivateModulesTest(unittest.TestCase):
    def fixture(self, directory):
        root = Path(directory)
        env_file, output_file = root / 'env', root / 'outputs'
        env_file.touch()
        output_file.touch()
        home_dir = root / 'home'
        home_dir.mkdir()
        (home_dir / '.gitconfig').write_text('[user]\nname = preserved\n')
        env = dict(os.environ, RUNNER_TEMP=str(root), GITHUB_ENV=str(env_file),
                   GITHUB_OUTPUT=str(output_file), MODULE_READ_TOKEN=TOKEN, HOME=str(home_dir))
        return root, env

    def test_git_header_scope_and_private_file(self):
        with tempfile.TemporaryDirectory(prefix='iam module test ') as directory:
            root, env = self.fixture(directory)
            result = subprocess.run(['bash', str(SETUP)], env=env, capture_output=True, text=True, check=True)
            config = Path((root / 'outputs').read_text().strip().split('=', 1)[1])
            self.assertEqual(stat.S_IMODE(config.stat().st_mode), 0o600)
            self.assertEqual((root / 'home/.gitconfig').read_text(), '[user]\nname = preserved\n')
            self.assertNotIn(TOKEN, result.stdout + result.stderr)
            self.assertNotIn(TOKEN, (root / 'env').read_text())
            expected = 'AUTHORIZATION: basic ' + base64.b64encode(('x-access-token:' + TOKEN).encode()).decode()
            for suffix in ('', '.git', '/info/refs', '.git/info/refs'):
                value = subprocess.run(['git', 'config', '--file', str(config), '--get-urlmatch',
                                        'http.extraheader', 'https://github.com/FangcunMount/reliable-messaging' + suffix],
                                       capture_output=True, text=True, check=True)
                self.assertEqual(value.stdout.strip(), expected)
            for url in ('https://github.com/FangcunMount/iam',
                        'https://github.com/FangcunMount/reliable-messaging-other',
                        'http://github.com/FangcunMount/reliable-messaging',
                        'https://other.example/FangcunMount/reliable-messaging'):
                value = subprocess.run(['git', 'config', '--file', str(config), '--get-urlmatch', 'http.extraheader', url],
                                       capture_output=True, text=True)
                self.assertEqual(value.returncode, 1)
                self.assertEqual(value.stdout, '')

    def test_missing_token_creates_no_auth_file(self):
        with tempfile.TemporaryDirectory() as directory:
            root, env = self.fixture(directory)
            env.pop('MODULE_READ_TOKEN')
            result = subprocess.run(['bash', str(SETUP)], env=env, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(list(root.glob('iam-module-auth.*')), [])
            self.assertEqual((root / 'outputs').read_text(), '')

    def test_config_failure_removes_partial_file(self):
        with tempfile.TemporaryDirectory() as directory:
            root, env = self.fixture(directory)
            bin_dir = root / 'bin'
            bin_dir.mkdir()
            git = bin_dir / 'git'
            git.write_text('#!/bin/sh\nexit 9\n')
            git.chmod(0o700)
            env['PATH'] = str(bin_dir) + os.pathsep + env['PATH']
            result = subprocess.run(['bash', str(SETUP)], env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode, 9)
            self.assertEqual(list(root.glob('iam-module-auth.*')), [])
            self.assertEqual((root / 'outputs').read_text(), '')

    def test_image_build_passes_secret_path_not_contents(self):
        with tempfile.TemporaryDirectory(prefix='iam image test ') as directory:
            root = Path(directory)
            config = root / 'private config'
            config.write_text(TOKEN)
            capture = root / 'arguments'
            docker = root / 'docker'
            docker.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$CAPTURE"\n')
            docker.chmod(0o700)
            env = dict(os.environ, PATH=str(root)+os.pathsep+os.environ['PATH'], CAPTURE=str(capture),
                       PRIVATE_MODULE_GIT_CONFIG=str(config), DOCKER_REGISTRY='registry.example',
                       DOCKER_REPOSITORY='fixture', DEPLOY_REF='test', DEPLOY_SHA='a'*40,
                       WWW_UID='2000', WWW_GID='2000')
            subprocess.run(['sh', str(ROOT / 'scripts/cd/build-image.sh')], env=env, check=True, capture_output=True)
            arguments = capture.read_text().splitlines()
            self.assertEqual(arguments[arguments.index('--secret')+1], 'id=private_git_config,src='+str(config))
            self.assertNotIn(TOKEN, capture.read_text())
            capture.unlink()
            env.pop('PRIVATE_MODULE_GIT_CONFIG')
            result = subprocess.run(['sh', str(ROOT / 'scripts/cd/build-image.sh')], env=env, capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(capture.exists())


if __name__ == '__main__':
    unittest.main()
