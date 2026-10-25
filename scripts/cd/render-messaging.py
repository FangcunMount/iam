"""Render only the reviewed IAM production transaction binding; no I/O clients."""

import json
import os
from pathlib import Path
import re
import sys


def production_binding(environment):
    if environment.get("IAM_MESSAGING_PROVIDER") != "rocketmq":
        raise ValueError("Production requires explicit RocketMQ transaction provider")
    for retired in ("IAM_RELIABLE_MESSAGING_ENABLED", "IAM_NSQ_CONSUMER_SDK_ENABLED"):
        if environment.get(retired, "false") != "false":
            raise ValueError("Retired production messaging selection is not supported")
    endpoint = environment.get("ROCKETMQ_ENDPOINT", "")
    if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9.-]*:[0-9]{1,5}", endpoint) or not 1 <= int(endpoint.rsplit(":", 1)[1]) <= 65535:
        raise ValueError("Explicit RocketMQ host:port required")
    instance = environment.get("ROCKETMQ_INSTANCE_ID", "")
    if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}", instance):
        raise ValueError("Stable IAM instance required")
    revision = environment.get("ROCKETMQ_BINDING_REVISION", "")
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]{0,63}", revision):
        raise ValueError("Immutable messaging file revision required")
    identity = {}
    for source, target in (("ROCKETMQ_STORE_ID", "store_id"), ("ROCKETMQ_WRITER_EPOCH", "writer_epoch")):
        value = environment.get(source, "")
        if not value.strip() or not 1 <= len(value.encode("utf-8")) <= 128 or "\x00" in value:
            raise ValueError("Explicit original transaction identity required")
        identity[target] = value  # Do not strip, case fold, regenerate or interpolate.
    admission = environment.get("ROCKETMQ_PUBLISH_ENABLED", "false")
    if admission not in {"true", "false"}:
        raise ValueError("Invalid publishing admission")
    identity.update(route_revision="m3-1", limits={
        "sends": 8, "branches": 16, "check_workers": 4, "check_queue": 32,
        "reconcile_batch": 100, "check_timeout": "2s", "rpc_timeout": "5s",
        "lease": "60s", "reconcile_interval": "5s",
    })
    return {
        "provider": "rocketmq", "endpoint": endpoint,
        "credentials_file": "/etc/iam/rocketmq/credentials.json",
        "ca_file": "/etc/iam/rocketmq/ca.crt",
        "routes_file": "/etc/iam/rocketmq/routes.json",
        "instance_id": instance, "publish_enabled": admission == "true",
        "transactions": {"iam.mysql": identity},
    }


def main():
    try:
        value = production_binding(os.environ)
        if sys.argv[1:] == ["validate"]:
            return
        if len(sys.argv) != 3 or sys.argv[1] != "render":
            raise ValueError("Expected validate or render CONFIG")
        path = Path(sys.argv[2])
        text = path.read_text()
        # The repository owns this base profile. Never rewrite an existing owner
        # map or use line substitution to interpret user-provided YAML.
        if re.search(r"^messaging\s*:", text, re.MULTILINE):
            raise ValueError("Production base already contains messaging")
        path.write_text(text + "\n# Frozen production binding; host owns the original identity.\nmessaging: " + json.dumps(value, ensure_ascii=True) + "\n")
    except (ValueError, TypeError, OSError):
        raise SystemExit("Invalid final IAM messaging configuration; contents withheld") from None


if __name__ == "__main__":
    main()
