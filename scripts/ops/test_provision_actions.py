import copy
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('actions_provision', Path(__file__).with_name('reviewer-actions.py'))
a = importlib.util.module_from_spec(spec)
spec.loader.exec_module(a)


class ActionsProvisionTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.payload = self.root / 'payload'
        self.payload.mkdir()
        self.state = self.root / 'state'
        a.private_directory(self.state)
        self.value = {'mode': 'preflight', 'run_id': '123', 'run_attempt': '1',
            'code_sha': 'a' * 40, 'plan_id': '', 'approve_plan': '',
            'password': 'PrivateDummy-Password-123', 'token': 'dummy-token',
            'database': dict.fromkeys(a.DB_KEYS, 'dummy')}
        self.calls = []
        def execute(args, iam, qs):
            self.calls.append(args.mode)
            if args.mode == 'preflight':
                a.p.save(args.plan, {'fingerprint': 'b' * 64})
        self.mock = patch.object(a.p, 'execute', execute)
        self.mock.start()
        self.addCleanup(self.mock.stop)
        self.env = patch.dict(os.environ)
        self.env.start()
        self.addCleanup(self.env.stop)
    def run_operation(self):
        with patch('builtins.print'):
            a.execute(self.value, self.payload, self.state)
    def test_preflight_apply_verify_preserve_account_and_bind_source(self):
        self.run_operation()
        account = json.loads(a.p.private_read(self.state / 'account.json'))
        self.assertEqual(account['username'], 'review@mfangcunmount.com')
        self.assertEqual(account['user_id'], '10002')
        self.value.update(mode='apply', plan_id='123-1', approve_plan='b' * 64)
        self.run_operation()
        self.value['mode'] = 'verify'
        self.run_operation()
        self.assertEqual(self.calls, ['preflight', 'apply', 'verify'])
        self.assertEqual(json.loads(a.p.private_read(self.state / 'account.json')), account)
    def test_missing_secret_and_invalid_plan_never_execute(self):
        for key, replacement in [('password', ''), ('token', ''), ('run_id', '../path'), ('code_sha', 'main')]:
            value = copy.deepcopy(self.value)
            value[key] = replacement
            with self.assertRaises(a.p.Stopped):a.validate(value)
        self.value.update(mode='apply', plan_id='../../other', approve_plan='b'*64)
        with self.assertRaises(a.p.Stopped):self.run_operation()
        self.assertFalse(self.calls)
    def test_source_change_refuses_apply(self):
        self.run_operation()
        self.value.update(mode='apply', plan_id='123-1', approve_plan='b'*64, code_sha='c'*40)
        with self.assertRaises(a.p.Stopped):self.run_operation()
        self.assertEqual(self.calls, ['preflight'])
    def test_changed_password_is_not_overwritten(self):
        self.run_operation()
        original = (self.state/'account.json').read_bytes()
        self.value['password'] = 'Changed-Private-Password-123'
        with self.assertRaises(a.p.Stopped):self.run_operation()
        self.assertEqual((self.state/'account.json').read_bytes(), original)
    def test_run_removes_transient_secrets_even_on_failure(self):
        a.p.save(self.payload/'private.json', self.value)
        with patch.object(Path, 'home', return_value=self.root), patch.object(a, 'execute', side_effect=a.p.Stopped('injected')):
            with self.assertRaises(a.p.Stopped):a.run(self.payload)
        self.assertFalse((self.payload/'private.json').exists())
    def test_private_state_rejects_symlinks(self):
        link = self.root/'link'
        link.symlink_to(self.state)
        with self.assertRaises(a.p.Stopped):a.private_directory(link)
    def test_pack_does_not_expose_credentials(self):
        env = {'REVIEWER_MODE':'preflight', 'GITHUB_RUN_ID':'123','GITHUB_RUN_ATTEMPT':'1',
               'GITHUB_SHA':'a'*40,'REVIEWER_PASSWORD':self.value['password'],'REVIEWER_TOKEN':self.value['token']}
        env.update({'IAM_APISERVER_MYSQL_'+k:v for k,v in self.value['database'].items()})
        with patch.dict(os.environ, env), patch('builtins.print') as output:
            a.pack(self.payload/'private.json')
        logged = str(output.call_args_list)
        self.assertNotIn(self.value['password'], logged)
        self.assertNotIn(self.value['token'], logged)
        self.assertEqual(json.loads(a.p.private_read(self.payload/'private.json'))['password'], self.value['password'])


if __name__ == '__main__':
    unittest.main()
