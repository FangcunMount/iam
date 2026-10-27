"""Exercise the shell lock with real descriptors and kernel flock, without sudo.

The stat adapter projects fixture ownership to root; modes, identities, reads
and contention use the real filesystem. Production UID checks are unchanged.
"""
import fcntl
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


class DeployLockTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.lockdir = self.root / "retention"
        self.lockdir.mkdir(mode=0o755)
        self.lockdir.chmod(0o755)
        self.lock = self.lockdir / "deploy.lock"
        self.lock.write_text("original inode\n")
        self.lock.chmod(0o666)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        adapters = {
            "stat": '''import os,stat,sys
fmt,path=sys.argv[-2:]
i=os.fstat(9) if path=="/proc/self/fd/9" else os.stat(path,follow_symlinks=False)
print(("0:"+format(stat.S_IMODE(i.st_mode),"o")) if fmt=="%u:%a" else str(i.st_dev)+":"+str(i.st_ino))
''',
            "flock": '''import fcntl,sys,time
until=time.monotonic()+0.3
while True:
 try: fcntl.flock(int(sys.argv[-1]),fcntl.LOCK_EX|fcntl.LOCK_NB);break
 except BlockingIOError:
  if time.monotonic()>=until:sys.exit(1)
  time.sleep(0.01)
''',
        }
        for name, code in adapters.items():
            p = self.bin / name
            p.write_text("#!/usr/bin/env python3\n" + code)
            p.chmod(0o755)
        helper = Path(__file__).with_name("image-retention.sh").read_text()
        self.helper = self.root / "helper.sh"
        self.helper.write_text(helper.replace("/var/lib/fangcun-image-retention", str(self.lockdir)))

    def run_helper(self):
        env = dict(os.environ, PATH=str(self.bin) + os.pathsep + os.environ["PATH"])
        return subprocess.run([
            "bash", "-c", '''set -euo pipefail
source "$1"
sudo_probe() {
  case "$*" in
    'docker ps -aq') echo original-container ;;
    'docker inspect --format {{.Image}} original-container') echo original-image ;;
    *) echo 'unapproved sudo command' >&2; return 91 ;;
  esac
}
SUDO=sudo_probe
acquire_image_deploy_lock
test "${RETENTION_PREVIOUS_IDS[*]}" = '--protect-image-id original-image'
echo locked
''', "fixture", str(self.helper),
        ], env=env, text=True, capture_output=True, timeout=5)

    def test_existing_lock_is_borrowed_without_metadata_or_content_changes(self):
        before = self.lock.stat()
        result = self.run_helper()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("locked", result.stdout)
        after = self.lock.stat()
        self.assertEqual((before.st_ino, before.st_mode, before.st_mtime_ns),
                         (after.st_ino, after.st_mode, after.st_mtime_ns))
        self.assertEqual(self.lock.read_text(), "original inode\n")

    def test_missing_lock_is_refused_without_creation(self):
        self.lock.unlink()
        self.assertNotEqual(self.run_helper().returncode, 0)
        self.assertFalse(self.lock.exists())

    def test_symlink_is_refused_without_touching_target(self):
        target = self.root / "protected"
        target.write_text("protected")
        self.lock.unlink()
        self.lock.symlink_to(target)
        self.assertNotEqual(self.run_helper().returncode, 0)
        self.assertEqual(target.read_text(), "protected")

    def test_permission_drift_is_refused_without_repair(self):
        self.lock.chmod(0o644)
        self.assertNotEqual(self.run_helper().returncode, 0)
        self.assertEqual(self.lock.stat().st_mode & 0o777, 0o644)

    def test_original_inode_excludes_competitor(self):
        with self.lock.open("r") as held:
            fcntl.flock(held.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            result = self.run_helper()
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn("locked", result.stdout)
        self.assertEqual(self.run_helper().returncode, 0)


if __name__ == "__main__":
    unittest.main()
