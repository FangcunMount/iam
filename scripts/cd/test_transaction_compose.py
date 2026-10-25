"""Required CI proof of the actual generated IAM package through Docker Compose."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class ProductionCompose(unittest.TestCase):
    def test_final_package_keeps_owner_files_and_original_resources(self):
        with tempfile.TemporaryDirectory(prefix="iam-m4-compose-") as temporary:
            package = Path(temporary) / "package"
            environment = {
                **os.environ,
                "SERVICE": "apiserver",
                "DEPLOY_PACKAGE_DIR": str(package),
                "DEPLOY_PACKAGE": str(Path(temporary) / "package.tar.gz"),
                "IAM_MESSAGING_PROVIDER": "rocketmq",
                "ROCKETMQ_ENDPOINT": "broker.invalid:8081",
                "ROCKETMQ_INSTANCE_ID": "iam-fixture",
                "ROCKETMQ_STORE_ID": "Original-$Primary",
                "ROCKETMQ_WRITER_EPOCH": "Original-Writer-Epoch",
                "ROCKETMQ_BINDING_REVISION": "m4-iam-fixture",
                "ROCKETMQ_PUBLISH_ENABLED": "false",
                "MYSQL_HOST": "mysql.invalid", "MYSQL_USERNAME": "synthetic",
                "MYSQL_PASSWORD": "synthetic", "MYSQL_DBNAME": "iam",
                "MYSQL_TLS_MODE": "VERIFY_IDENTITY", "MYSQL_TLS_CA_REF": "a" * 40,
                "REDIS_HOST": "redis.invalid", "REDIS_PORT": "6379",
                "IPD_ENCRYPTION_KEY": "synthetic", "SEED_MOCK_AUTH_ENABLED": "false",
                "WWW_UID": "2000", "WWW_GID": "2000", "IMAGE_TAG": "a" * 40,
            }
            subprocess.run(["bash", "scripts/cd/prepare-package.sh"], cwd=ROOT, env=environment,
                           check=True, capture_output=True, text=True)
            result = subprocess.run([
                "docker", "compose", "--env-file", str(package / "configs/env/config.prod.env"),
                "-f", str(package / "build/docker/docker-compose.prod.yml"), "config", "--format", "json",
            ], cwd=ROOT, env=environment, check=True, capture_output=True, text=True)
            config = json.loads(result.stdout)
            self.assertEqual(set(config["services"]), {"iam-apiserver"})
            service = config["services"]["iam-apiserver"]
            self.assertEqual(int(service["mem_limit"]), 1024 * 1024 * 1024)
            self.assertEqual(float(service["cpus"]), 1)
            self.assertEqual(service["user"], "2000:2000")
            mounts = [m for m in service["volumes"] if m["target"].startswith("/etc/iam/rocketmq/")]
            self.assertEqual({m["target"].rsplit("/", 1)[1] for m in mounts},
                             {"credentials.json", "ca.crt", "routes.json"})
            for mount in mounts:
                self.assertTrue(mount["read_only"])
                # Compose omits false fields from its normalized JSON model.
                self.assertFalse(mount["bind"].get("create_host_path", False))
                self.assertEqual(mount["source"], "/data/infra/iam-messaging/versions/m4-iam-fixture/rocketmq/" + mount["target"].rsplit("/", 1)[1])
            self.assertFalse(any("NSQ_" in k for k in service["environment"]))
            self.assertEqual(service["environment"]["IAM_APISERVER_MYSQL_TLS_MODE"], "VERIFY_IDENTITY")
            self.assertEqual(service["stop_grace_period"], "1m0s")


if __name__ == "__main__":
    unittest.main()
