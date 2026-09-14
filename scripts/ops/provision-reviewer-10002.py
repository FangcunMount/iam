#!/usr/bin/env python3
"""Provision reviewer 10002: reserved IAM signup, reviewed role assignments, QS enrollment.

No production writes unless mode=apply and --approve-plan matches the saved plan.
Run beside the rebuilt iam-maintenance binary with its IAM database environment.
"""
import argparse
import fcntl
import getpass
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid

REFERENCE = '10001'
TARGET = '10002'
ALLOWED_ROLES = {'platform_admin', 'iam_admin', 'qs:admin'}


class Stopped(Exception):
    pass


def digest(value):
    return hashlib.sha256(json.dumps(value, ensure_ascii=False, sort_keys=True,
                                     separators=(',', ':')).encode()).hexdigest()


def private_read(path):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'r') as f:
        info = os.fstat(f.fileno())
        if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) & 0o077 or info.st_size > 4_000_000:
            raise Stopped('Input must be a private regular file (0600), at most 4 MB')
        return f.read()


def save(path, value):
    # Exclusive receipts: never overwrite evidence, credentials, or an older plan.
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as f:
        json.dump(value, f, ensure_ascii=False, indent=2)
        f.flush()
        os.fsync(f.fileno())
    directory = os.open(Path(path).parent, os.O_RDONLY)
    try:
        os.fsync(directory)
    finally:
        os.close(directory)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise Stopped('HTTP redirect refused; use the final trusted API URL')


class API:
    def __init__(self, base, token):
        parsed = urllib.parse.urlsplit(base)
        if parsed.scheme != 'https' or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
            raise Stopped('A trusted HTTPS API base URL without credentials is required')
        self.base = base.rstrip('/')
        self.token = token
        self.opener = urllib.request.build_opener(NoRedirect())

    def request(self, method, path, body=None):
        if not path.startswith('/') or path.startswith('//'):
            raise Stopped('Invalid API path')
        req = urllib.request.Request(self.base + path,
            data=None if body is None else json.dumps(body).encode(), method=method,
            headers={'Authorization': 'Bearer ' + self.token, 'Content-Type': 'application/json'})
        try:
            with self.opener.open(req, timeout=45) as response:
                raw = response.read(4_000_001)
                if len(raw) > 4_000_000:
                    raise Stopped('API response too large')
                payload = json.loads(raw)
        except urllib.error.HTTPError as error:
            raise Stopped(f'API HTTP {error.code}; no automatic retry, inspect receipts') from None
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError):
            raise Stopped('API result uncertain; rerun verification before another mutation') from None
        if not isinstance(payload, dict) or payload.get('code') not in (0, 200):
            raise Stopped('API rejected request; response body omitted to protect private data')
        return payload

    def rows(self, path):
        payload = self.request('GET', path)
        data = payload.get('data')
        if not isinstance(data, list) or payload.get('total', len(data)) != len(data):
            raise Stopped('Incomplete IAM list; refusing a partial role inventory')
        return data

    def assignments(self, user_id):
        rows = self.rows('/authz/assignments/subject?subject_type=user&subject_id=' + user_id)
        if any(str(a['subject_id']) != user_id or a['subject_type'] != 'user' for a in rows):
            raise Stopped('Unexpected assignment owner')
        ids = [str(a['role_id']) for a in rows]
        if len(ids) != len(set(ids)):
            raise Stopped('Duplicate/scoped assignments require separate review')
        return rows

    def operators(self):
        rows = []
        for page in range(1, 1001):
            data = self.request('GET', f'/operators?page={page}&page_size=100')['data']
            items = data.get('items', [])
            rows.extend(items)
            if len(rows) == data['total']:
                if len({str(r['id']) for r in rows}) != len(rows):
                    raise Stopped('QS operator pagination drift')
                return rows
            if not items or len(rows) > data['total']:
                break
        raise Stopped('QS operator inventory incomplete')


def reference_snapshot(iam, qs):
    assignments = iam.assignments(REFERENCE)
    roles = []
    for a in assignments:
        role_id = str(a['role_id'])
        role = iam.request('GET', '/authz/roles/' + role_id)['data']
        if role['name'] not in ALLOWED_ROLES:
            raise Stopped('10001 has roles outside the reviewed administrator set; inspect manually')
        grants = iam.rows('/authz/roles/' + role_id + '/grants')
        roles.append({'id': role_id, 'name': role['name'], 'definition': role,
                      'grants': sorted(grants, key=lambda g: str(g['id']))})
    if not {'platform_admin', 'qs:admin'} <= {r['name'] for r in roles}:
        raise Stopped('Reference must have platform_admin and qs:admin')
    refs = [r for r in qs.operators() if str(r['user_id']) == REFERENCE]
    if len(refs) != 1 or not refs[0]['is_active'] or 'qs:admin' not in refs[0]['roles'] or refs[0]['authz_projection_pending']:
        raise Stopped('Reference QS administrator is missing, ambiguous, inactive, or not synchronized')
    return {'assignments': sorted(assignments, key=lambda a: str(a['id'])),
            'roles': sorted(roles, key=lambda r: r['id']),
            'org_id': str(refs[0]['org_id']), 'operator_id': str(refs[0]['id'])}


def account(args, mode, fingerprint=None):
    receipt = args.directory / ('iam-' + mode + '-' + uuid.uuid4().hex + '.json')
    command = [args.maintenance, 'account-provision', mode, '--input',
               str(args.directory / 'account.json'), '--report', str(receipt)]
    if fingerprint:
        command += ['--fingerprint', fingerprint]
    # No secret or raw DB error is forwarded to the terminal.
    result = subprocess.run(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
    if result.returncode:
        raise Stopped('IAM provisioning stopped; inspect its private receipt; no automatic password reset')
    report = json.loads(private_read(receipt))['report']
    if mode == 'apply' and report.get('user_id') != TARGET:
        raise Stopped('IAM returned an unexpected user ID')
    return report


def observe_target(iam, qs, source, name):
    actual = {str(a['role_id']) for a in iam.assignments(TARGET)}
    expected = {r['id'] for r in source['roles']}
    if not actual <= expected:
        raise Stopped('Target has unplanned roles; no revocation or overwrite will be attempted')
    operators = [o for o in qs.operators() if str(o['user_id']) == TARGET]
    if len(operators) > 1 or (operators and (str(operators[0]['org_id']) != source['org_id'] or
                           operators[0]['name'] != name or not operators[0]['is_active'])):
        raise Stopped('Target QS membership differs from the reviewed plan')
    return actual, operators


def execute(args, iam, qs):
    value = json.loads(private_read(args.directory / 'account.json'))
    if value.get('user_id') != TARGET or value.get('actor_id') != REFERENCE:
        raise Stopped('This script only provisions 10002 with reference 10001')
    public = {k: v for k, v in value.items() if k != 'password'}
    source = reference_snapshot(iam, qs)
    current = account(args, 'preflight')
    actual, operators = observe_target(iam, qs, source, value['name'])
    if current['state'] == 'pending' and (actual or operators):
        raise Stopped('New target ID already has authorization or QS membership')
    basis = {'account': public, 'account_fingerprint': current['fingerprint'], 'reference': source,
             'iam_url': iam.base, 'qs_url': qs.base}
    if args.mode == 'preflight':
        plan = {'basis': basis, 'fingerprint': digest(basis)}
        save(args.plan, plan)
        print(json.dumps({'state': 'preflight', 'user_id': TARGET, 'username': value['username'],
            'roles': [r['name'] for r in source['roles']], 'org_id': source['org_id'],
            'fingerprint': plan['fingerprint'], 'plan': str(args.plan)}, ensure_ascii=False))
        return
    plan = json.loads(private_read(args.plan))
    if plan['basis'] != basis or plan['fingerprint'] != digest(basis):
        raise Stopped('Reference, endpoint, or account plan changed; stop and prepare a fresh reviewed plan')
    if args.mode == 'apply':
        if args.approve_plan != plan['fingerprint']:
            raise Stopped('Apply requires --approve-plan with the reviewed fingerprint (full reference admin access)')
        account(args, 'apply', current['fingerprint'])
        for role in source['roles']:
            if reference_snapshot(iam, qs) != source:
                raise Stopped('Reference authorization drift; partial completion may exist, inspect receipts')
            actual, operators = observe_target(iam, qs, source, value['name'])
            if role['id'] not in actual:
                intent = {'operation': 'grant_role', 'target': TARGET, 'role': role['name'],
                          'role_id': role['id'], 'plan': plan['fingerprint']}
                save(args.directory / ('intent-' + uuid.uuid4().hex + '.json'), intent)
                iam.request('POST', '/authz/assignments/grant',
                            {'subject_type': 'user', 'subject_id': TARGET, 'role_id': role['id']})
        actual, operators = observe_target(iam, qs, source, value['name'])
        if not operators:
            save(args.directory / ('intent-' + uuid.uuid4().hex + '.json'),
                 {'operation': 'qs_enroll', 'target': TARGET, 'plan': plan['fingerprint']})
            qs.request('POST', '/operators', {'user_id': TARGET, 'name': value['name'],
                'org_id': int(source['org_id']), 'roles': [], 'is_active': True})
    if reference_snapshot(iam, qs) != source:
        raise Stopped('Reference changed during execution; completion not confirmed')
    actual, operators = observe_target(iam, qs, source, value['name'])
    state = 'incomplete'
    if current['state'] == 'historical_completed' or args.mode == 'apply':
        if actual == {r['id'] for r in source['roles']} and operators:
            op = operators[0]
            if 'qs:admin' in op['roles'] and not op['authz_projection_pending']:
                state = 'provisioned'
            else:
                state = 'awaiting_projection'
    receipt = {'state': state, 'user_id': TARGET, 'plan': plan['fingerprint'],
               'role_ids': sorted(actual), 'operator_id': str(operators[0]['id']) if operators else None,
               'login_and_human_review': 'not_performed'}
    save(args.directory / ('verification-' + uuid.uuid4().hex + '.json'), receipt)
    print(json.dumps(receipt))
    if state != 'provisioned':
        raise Stopped('Not complete yet; rerun verify after inspecting state/projection')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['init', 'preflight', 'apply', 'verify'])
    parser.add_argument('--directory', required=True, type=lambda p: Path(p).absolute())
    parser.add_argument('--username', default='review@mfangcunmount.com')
    parser.add_argument('--maintenance', default='./iam-maintenance')
    parser.add_argument('--token-file', type=Path)
    parser.add_argument('--iam-url', default='https://iam.fangcunmount.cn/api/v4')
    parser.add_argument('--qs-url', default='https://qs.fangcunmount.cn/api/v1')
    parser.add_argument('--plan', type=Path)
    parser.add_argument('--approve-plan')
    args = parser.parse_args()
    if args.mode == 'init':
        if not sys.stdin.isatty():
            raise Stopped('Initialize in an interactive terminal to enter a private password')
        args.directory.mkdir(mode=0o700)
    if args.directory.is_symlink() or stat.S_IMODE(args.directory.stat().st_mode) & 0o077:
        raise Stopped('Working directory must be private (0700)')
    lock = os.open(args.directory / '.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.mode == 'init':
            password = getpass.getpass('新账号密码（20–128 字节，不显示）: ')
            if not 20 <= len(password.encode()) <= 128 or password.strip() != password or password != getpass.getpass('再次输入: '):
                raise Stopped('Password length or confirmation mismatch')
            save(args.directory / 'account.json', {'request_id': 'reviewer-10002-' + uuid.uuid4().hex,
                'actor_id': REFERENCE, 'user_id': TARGET, 'username': args.username,
                'name': '安全与产品审核员', 'reason': '创建与10001同级的独立管理员，用于安全与产品人工审核', 'password': password})
            print('Private account input prepared; no account or authorization created.')
            return
        if not args.token_file:
            raise Stopped('--token-file containing the existing authorized administrator access token is required')
        token = private_read(args.token_file).strip()
        if not token or any(c.isspace() for c in token):
            raise Stopped('Invalid private token input')
        args.plan = args.plan or args.directory / 'plan.json'
        execute(args, API(args.iam_url, token), API(args.qs_url, token))
    finally:
        os.close(lock)


if __name__ == '__main__':
    try:
        main()
    except (Stopped, OSError, KeyError, TypeError, ValueError, subprocess.SubprocessError) as error:
        print('STOP: ' + (str(error) if isinstance(error, Stopped) else type(error).__name__), file=sys.stderr)
        sys.exit(1)
