#!/usr/bin/env python3
"""Check repository facts that active documentation relies on."""

from __future__ import annotations

from pathlib import Path
import re
import subprocess
import sys

import yaml


ROOT = Path(__file__).resolve().parent.parent


def fail(message: str) -> None:
    raise AssertionError(message)


def load_yaml(relative: str) -> dict:
    with (ROOT / relative).open(encoding="utf-8") as handle:
        return yaml.safe_load(handle) or {}


def check_runtime_configuration() -> None:
    for relative in (
        "configs/apiserver.dev.yaml",
        "configs/apiserver.prod.yaml",
        "configs/suggest.dev.yaml",
        "configs/suggest.prod.yaml",
    ):
        config = load_yaml(relative)
        suggest = config.get("suggest", {})
        for retired in ("snapshot", "data_dir"):
            if retired in suggest:
                fail(f"{relative} still contains retired suggest.{retired}")

    dev = load_yaml("configs/apiserver.dev.yaml")
    prod = load_yaml("configs/apiserver.prod.yaml")
    required_rotation = {
        "automatic_enabled",
        "check_cron",
        "rotation_interval",
        "grace_period",
        "max_publishable_keys",
    }
    for relative, config, automatic in (
        ("configs/apiserver.dev.yaml", dev, False),
        ("configs/apiserver.prod.yaml", prod, True),
    ):
        rotation = config.get("jwks", {}).get("rotation", {})
        missing = required_rotation - rotation.keys()
        if missing:
            fail(f"{relative} is missing JWKS rotation keys: {sorted(missing)}")
        if rotation["automatic_enabled"] is not automatic:
            fail(f"{relative} has the wrong automatic rotation default")


def check_migrations() -> None:
    directory = ROOT / "internal/pkg/migration/migrations"
    up = {
        int(match.group(1))
        for path in directory.glob("*.up.sql")
        if (match := re.match(r"(\d{6})_", path.name))
    }
    down = {
        int(match.group(1))
        for path in directory.glob("*.down.sql")
        if (match := re.match(r"(\d{6})_", path.name))
    }
    if up != down:
        fail(f"migration up/down numbers differ: up-only={sorted(up-down)} down-only={sorted(down-up)}")
    if not up or max(up) != 16:
        fail(f"documented latest migration is 16, repository has {max(up) if up else 'none'}")
    migration = (directory / "000016_jwks_single_active_guard.up.sql").read_text(encoding="utf-8")
    for token in ("active_guard", "uk_jwks_keys_single_active"):
        if token not in migration:
            fail(f"migration 000016 is missing {token}")


def check_event_catalog() -> None:
    catalog = load_yaml("configs/events.yaml")
    topics = catalog.get("topics", {})
    events = catalog.get("events", {})
    if not topics or not events:
        fail("configs/events.yaml must declare topics and events")
    for name, event in events.items():
        topic = event.get("topic")
        if topic not in topics:
            fail(f"event {name} references missing topic {topic}")


def check_module_wiring() -> None:
    source = (ROOT / "internal/apiserver/container/bootstrap_modules.go").read_text(encoding="utf-8")
    for constructor in ("authn.NewAuthnModule()", "suggest.NewSuggestModule()"):
        if constructor not in source:
            fail(f"composition root no longer wires {constructor}")


def check_jwks_lifecycle_wiring() -> None:
    application = (
        ROOT / "internal/apiserver/container/authn/application.go"
    ).read_text(encoding="utf-8")
    scheduler = (
        ROOT / "internal/apiserver/container/authn/scheduler.go"
    ).read_text(encoding="utf-8")
    rest = (ROOT / "internal/apiserver/container/authn/rest.go").read_text(encoding="utf-8")
    handler = (
        ROOT
        / "internal/apiserver/transport/rest/authn/handler/jwks_admin_keys.go"
    ).read_text(encoding="utf-8")
    for token, source in (
        ("NewKeyLifecycleAppService", application),
        ("m.keyLifecycleApp", scheduler),
        ("caps.KeyLifecycleApp", rest),
        ("keyLifecycleApp.CreateAndActivate", handler),
    ):
        if token not in source:
            fail(f"JWKS lifecycle wiring is missing {token}")


def check_active_docs() -> None:
    forbidden = "待补证据"
    for path in (ROOT / "docs").rglob("*.md"):
        if "_archive" in path.parts:
            continue
        text = path.read_text(encoding="utf-8")
        if forbidden in text:
            fail(f"{path.relative_to(ROOT)} contains unowned evidence placeholder {forbidden}")
        if "索引和文件 snapshot 仍保存原始手机号" in text:
            fail(f"{path.relative_to(ROOT)} claims the retired Suggest file snapshot still exists")

    suggest_query = (
        ROOT / "docs/02-业务模块/05-Suggest/03-关键链路-SuggestProfile查询.md"
    ).read_text(encoding="utf-8")
    for required in ("原始手机号只存在于进程内索引", "不写入文件或日志"):
        if required not in suggest_query:
            fail(f"Suggest query documentation is missing current privacy fact: {required}")


def run_contract_check(script: str) -> None:
    subprocess.run(
        [sys.executable, str(ROOT / "scripts" / script)],
        cwd=ROOT,
        check=True,
    )


def main() -> int:
    checks = (
        check_runtime_configuration,
        check_migrations,
        check_event_catalog,
        check_module_wiring,
        check_jwks_lifecycle_wiring,
        check_active_docs,
    )
    try:
        for check in checks:
            check()
        run_contract_check("check-openapi-contracts.py")
        run_contract_check("check-route-contracts.py")
    except (AssertionError, subprocess.CalledProcessError) as error:
        print(f"docs facts failed: {error}", file=sys.stderr)
        return 1
    print("Documentation facts match configuration, migrations, events, routes, and module wiring.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
