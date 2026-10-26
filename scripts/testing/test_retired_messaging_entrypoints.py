"""Retired production-assembly proofs refuse before any host/dependency work."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]

class RetiredMessagingEntrypoints(unittest.TestCase):
    def test_proofs_reject_before_docker_build_git_or_temporary_files(self):
        for name in ('reliable-messaging-proof.sh', 'reliable-messaging-business-proof.sh'):
            for arguments in ([], ['/unused-sdk', '/unused-qs']):
                with self.subTest(script=name, arguments=arguments), tempfile.TemporaryDirectory() as raw:
                    root=Path(raw); marker=root/'external-call'
                    for command in ('docker','go','git','mktemp'):
                        p=root/command;p.write_text('#!/bin/sh\nprintf invoked >> "$REFUSAL_MARKER"\nexit 99\n');p.chmod(0o700)
                    env={**os.environ,'PATH':str(root)+os.pathsep+os.environ['PATH'], 'REFUSAL_MARKER':str(marker)}
                    result=subprocess.run(['bash',str(ROOT/'scripts/testing'/name),*arguments],env=env,capture_output=True,text=True,timeout=5)
                    self.assertEqual(result.returncode,2,result.stderr)
                    self.assertIn('retired',result.stderr.lower())
                    self.assertFalse(marker.exists(),'retired proof touched host/dependency commands')

if __name__=='__main__':unittest.main()
