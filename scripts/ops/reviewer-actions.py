#!/usr/bin/env python3
"""Private Actions transport for the fixed 10002 provisioning procedure."""
import argparse
import fcntl
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import sys
from types import SimpleNamespace

spec = importlib.util.spec_from_file_location('provision', Path(__file__).with_name('provision-reviewer-10002.py'))
p = importlib.util.module_from_spec(spec)
spec.loader.exec_module(p)

USERNAME = 'review@mfangcunmount.com'
DB_KEYS = ('HOST', 'PORT', 'USERNAME', 'PASSWORD', 'DATABASE')


def validate(value):
    if value.get('mode') not in ('preflight', 'apply', 'verify'):
        raise p.Stopped('Invalid operation')
    for key in ('run_id', 'run_attempt'):
        if not re.fullmatch(r'[1-9][0-9]*', value.get(key, '')):
            raise p.Stopped('Invalid Actions run identifier')
    if not re.fullmatch(r'[0-9a-f]{40}', value.get('code_sha', '')):
        raise p.Stopped('An exact source commit is required')
    if value['mode'] != 'preflight' and not re.fullmatch(r'[1-9][0-9]*-[1-9][0-9]*', value.get('plan_id', '')):
        raise p.Stopped('Select the preflight plan ID from its run output')
    if value['mode'] == 'apply' and not re.fullmatch(r'[0-9a-f]{64}', value.get('approve_plan', '')):
        raise p.Stopped('Apply requires the reviewed preflight fingerprint')
    password = value.get('password', '')
    if not 20 <= len(password.encode()) <= 128 or password.strip() != password:
        raise p.Stopped('IAM_REVIEWER_10002_PASSWORD must contain 20-128 bytes without edge whitespace')
    token = value.get('token', '')
    if not token or any(c.isspace() for c in token):
        raise p.Stopped('IAM_REVIEWER_PROVISION_TOKEN is missing or malformed')
    if any(not value.get('database', {}).get(k) for k in DB_KEYS):
        raise p.Stopped('IAM database Secrets are incomplete')


def pack(destination):
    value = {key: os.environ.get(env, '') for key, env in {
        'mode': 'REVIEWER_MODE', 'run_id': 'GITHUB_RUN_ID', 'run_attempt': 'GITHUB_RUN_ATTEMPT',
        'code_sha': 'GITHUB_SHA', 'plan_id': 'REVIEWER_PLAN_ID', 'approve_plan': 'REVIEWER_APPROVE_PLAN',
        'password': 'REVIEWER_PASSWORD', 'token': 'REVIEWER_TOKEN'}.items()}
    value['database'] = {k: os.environ.get('IAM_APISERVER_MYSQL_' + k, '') for k in DB_KEYS}
    validate(value)
    p.save(destination, value)
    print('Private inputs prepared; no secrets printed or uploaded as artifacts.')


def private_directory(path):
    path.mkdir(mode=0o700, exist_ok=True)
    info = path.lstat()
    if not stat.S_ISDIR(info.st_mode) or stat.S_IMODE(info.st_mode) & 0o077 or info.st_uid != os.getuid():
        raise p.Stopped('State directory must be owned by the maintenance user with mode 0700')


def execute(value, payload, directory):
    validate(value)
    account_path = directory / 'account.json'
    desired = {'request_id': 'reviewer-10002-actions', 'actor_id': '10001', 'user_id': '10002',
               'username': USERNAME, 'name': '安全与产品审核员',
               'reason': '创建与10001同级的独立管理员，用于安全与产品人工审核', 'password': value['password']}
    if account_path.exists() or account_path.is_symlink():
        if json.loads(p.private_read(account_path)) != desired:
            raise p.Stopped('Retained account input differs; do not overwrite identity or password')
    elif value['mode'] == 'preflight':
        p.save(account_path, desired)
    else:
        raise p.Stopped('Run preflight first; no retained account input exists')
    plan_id = value['run_id'] + '-' + value['run_attempt'] if value['mode'] == 'preflight' else value['plan_id']
    plans = directory / 'plans'
    private_directory(plans)
    binding_path = plans / (plan_id + '.source.json')
    binding = {'code_sha': value['code_sha'], 'username': USERNAME, 'user_id': '10002'}
    if value['mode'] == 'preflight':
        p.save(binding_path, binding)
    elif json.loads(p.private_read(binding_path)) != binding:
        raise p.Stopped('Source commit differs from preflight; run a fresh preflight before applying')
    args = SimpleNamespace(mode=value['mode'], directory=directory,
        maintenance=str(payload / 'iam-maintenance'), plan=plans / (plan_id + '.json'),
        approve_plan=value['approve_plan'])
    for key in DB_KEYS:
        os.environ['IAM_APISERVER_MYSQL_' + key] = value['database'][key]
    print(json.dumps({'operation': value['mode'], 'plan_id': plan_id, 'code_sha': value['code_sha']}))
    p.execute(args, p.API('https://iam.fangcunmount.cn/api/v4', value['token']),
              p.API('https://qs.fangcunmount.cn/api/v1', value['token']))


def run(payload):
    private_input = payload / 'private.json'
    try:
        value = json.loads(p.private_read(private_input))
        directory = Path.home() / '.iam-reviewer-10002'
        private_directory(directory)
        lock = os.open(directory / '.lock', os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            execute(value, payload, directory)
        finally:
            os.close(lock)
    finally:
        private_input.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='mode', required=True)
    sub.add_parser('pack').add_argument('--output', type=Path, required=True)
    sub.add_parser('run').add_argument('--payload', type=Path, required=True)
    args = parser.parse_args()
    if args.mode == 'pack':
        pack(args.output)
    else:
        run(args.payload.absolute())


if __name__ == '__main__':
    try:
        main()
    except (p.Stopped, OSError, ValueError, KeyError, TypeError) as error:
        print('STOP: ' + (str(error) if isinstance(error, p.Stopped) else type(error).__name__), file=sys.stderr)
        sys.exit(1)
