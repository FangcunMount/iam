import argparse
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('provision', Path(__file__).with_name('provision-reviewer-10002.py'))
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)


class FakeAPI:
    base = 'https://trusted.invalid/api'
    def __init__(self):
        self.target = []
        self.members = [{'id': 'operator-1', 'user_id': '10001', 'org_id': '1',
                         'is_active': True, 'roles': ['qs:admin'], 'authz_projection_pending': False}]
        self.roles = {str(i): name for i, name in enumerate(sorted(p.ALLOWED_ROLES), 1)}
        self.scope_missing = False
        self.posts = []
        self.fail_after = None
    def assignments(self, user):
        if user == '10001':
            return [{'id': 'a' + i, 'role_id': i, 'subject_id': user, 'subject_type': 'user'} for i in self.roles]
        return copy.deepcopy(self.target)
    def operators(self): return copy.deepcopy(self.members)
    def rows(self, path): return [{'id': 'grant-' + path, 'active': True}]
    def request(self, method, path, body=None):
        if method == 'GET' and path.endswith('/authorization-scope'):
            return {'data': {'operator_id': path.split('/')[-2],
                'unconfigured_assignments': [{'role_id': '1', 'role_name':'qs:admin', 'scope':None, 'management_protection':'protected'}] if self.scope_missing else [],
                'assignments': [{'role_id': '1', 'management_protection': 'protected',
                                 'scope': {'org_id':'1', 'kind':'all_stores', 'store_ids':[]}}]}}
        if method == 'GET':
            i = path.split('/')[-1]
            return {'data': {'id': i, 'name': self.roles[i], 'management_protection': 'protected'}}
        self.posts.append((path, body))
        if path == '/authz/assignments/grant':
            self.target.append({'id': 't'+body['role_id'], **body})
        elif path == '/operators':
            self.members.append({'id': 'operator-2', **body, 'roles': ['qs:admin'], 'authz_projection_pending': False})
        else: raise AssertionError(path)
        if self.fail_after == path:
            self.fail_after = None
            raise p.Stopped('simulated lost response')
        return {'data': {}}


class ProvisionTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.directory = Path(self.tmp.name)
        self.api = FakeAPI()
        self.created = False
        self.creations = 0
        self.value = {'request_id': 'test-10002', 'actor_id': '10001', 'user_id': '10002',
                      'name': '安全与产品审核员', 'username': 'review@mfangcunmount.com',
                      'reason': 'authorized test', 'password': 'test-password-private-123'}
        p.save(self.directory/'account.json', self.value)
        self.args = argparse.Namespace(directory=self.directory, plan=self.directory/'plan.json',
                                      mode='preflight', approve_plan=None)
        def account(args, mode, fingerprint=None):
            if mode == 'apply' and not self.created:
                self.created = True
                self.creations += 1
            return {'state': 'historical_completed' if self.created else 'pending',
                    'fingerprint': 'account-fingerprint', 'user_id': '10002' if self.created else ''}
        self.mock = patch.object(p, 'account', account)
        self.mock.start()
        self.addCleanup(self.mock.stop)
    def execute(self):
        with patch('builtins.print'):
            return p.execute(self.args, self.api, self.api)
    def plan(self):
        self.execute()
        plan = json.loads((self.directory/'plan.json').read_text())
        self.assertNotIn('password', json.dumps(plan))
        self.assertFalse(self.api.posts)
        self.assertEqual(self.creations, 0)
        self.args.approve_plan = plan['fingerprint']
        self.args.mode = 'apply'
    def test_preflight_apply_verify_and_repeat_never_duplicate(self):
        self.plan(); self.execute(); self.execute()
        self.args.mode='verify'; self.execute()
        self.assertEqual(self.creations, 1)
        self.assertEqual(len(self.api.posts), 4)
        self.assertEqual(len(self.api.target), 3)
        self.assertEqual(len(self.api.members), 2)
    def test_apply_requires_exact_reviewed_fingerprint(self):
        self.plan(); self.args.approve_plan='wrong'
        with self.assertRaises(p.Stopped): self.execute()
        self.assertFalse(self.api.posts);self.assertFalse(self.created)
    def test_reference_drift_stops_before_mutation(self):
        self.plan(); self.api.members[0]['org_id']='2'
        with self.assertRaises(p.Stopped): self.execute()
        self.assertFalse(self.api.posts);self.assertFalse(self.created)
    def test_unplanned_target_privilege_is_not_replaced(self):
        self.plan();self.api.target=[{'role_id':'unexpected'}]
        with self.assertRaises(p.Stopped):self.execute()
        self.assertFalse(self.api.posts)
    def test_role_write_lost_response_is_reconciled_before_resume(self):
        self.plan();self.api.fail_after='/authz/assignments/grant'
        with self.assertRaises(p.Stopped):self.execute()
        self.assertEqual(len(self.api.target),1)
        self.execute()
        self.assertEqual(len(self.api.posts),4)
        self.assertEqual(self.creations,1)
    def test_qs_write_lost_response_is_not_repeated(self):
        self.plan();self.api.fail_after='/operators'
        with self.assertRaises(p.Stopped):self.execute()
        self.execute()
        self.assertEqual(len(self.api.posts),4)
    def test_reference_unknown_role_is_rejected(self):
        self.api.roles['4']='another_business_role'
        with self.assertRaises(p.Stopped):self.execute()
        self.assertFalse(self.api.posts)
    def test_non_reference_qs_membership_rejected(self):
        self.api.members.append({'id':'x','user_id':'10002','org_id':'999','name':self.value['name'],'is_active':True})
        with self.assertRaises(p.Stopped):self.execute()
    def test_stray_qs_membership_after_plan_stops_before_signup(self):
        self.plan()
        self.api.members.append({'id':'x','user_id':'10002','org_id':'1',
                                 'name':self.value['name'],'is_active':True})
        with self.assertRaises(p.Stopped):self.execute()
        self.assertFalse(self.created)
        self.assertFalse(self.api.posts)
    def test_projection_delay_is_incomplete_and_verify_never_writes(self):
        self.plan(); self.execute()
        self.api.members[1]['authz_projection_pending'] = True
        self.api.members[1]['roles'] = None
        self.args.mode = 'verify'
        with self.assertRaises(p.Stopped):self.execute()
        receipts = [json.loads(f.read_text()) for f in self.directory.glob('verification-*.json')]
        self.assertIn('awaiting_projection', {r['state'] for r in receipts})
        self.assertEqual(len(self.api.posts), 4)
        self.api.members[1]['authz_projection_pending'] = False
        self.api.members[1]['roles'] = ['qs:admin']
        self.execute()
        self.assertEqual(len(self.api.posts), 4)
    def test_list_pagination_requires_complete_unique_inventory(self):
        api = p.API('https://trusted.invalid/api', 'private')
        api.request = lambda *args: {'code': 0, 'data': [], 'total': 1}
        with self.assertRaises(p.Stopped):api.rows('/authz/assignments/subject')
        replies = iter([{'data': {'items': [{'id':'1'}], 'total':2}},
                        {'data': {'items': [{'id':'1'}], 'total':2}}])
        api.request = lambda *args: next(replies)
        with self.assertRaises(p.Stopped):api.operators()
    def test_private_inputs_and_receipts(self):
        path=self.directory/'secret';path.write_text('private');path.chmod(0o644)
        with self.assertRaises(p.Stopped):p.private_read(path)
        path.chmod(0o600);self.assertEqual(p.private_read(path),'private')
        with self.assertRaises(FileExistsError):p.save(path,{'overwrite':True})
        link=self.directory/'symlink';link.symlink_to(path)
        with self.assertRaises(OSError):p.private_read(link)
    def test_missing_scope_never_reports_provisioned(self):
        self.plan()
        self.api.scope_missing = True
        with self.assertRaisesRegex(p.Stopped, 'scopes are missing'):
            self.execute()
        self.assertFalse(list(self.directory.glob('verification-*.json')))

    def test_https_and_redirect_boundaries(self):
        for url in ['http://example.org/api','https://user:secret@example.org','https://example.org?token=x']:
            with self.assertRaises(p.Stopped):p.API(url,'private')
        with self.assertRaises(p.Stopped):p.NoRedirect().redirect_request(None,None,302,'',{},'https://other.invalid')

if __name__=='__main__':unittest.main()
