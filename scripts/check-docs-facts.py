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
    if not up or max(up) != 18:
        fail(f"documented latest migration is 18, repository has {max(up) if up else 'none'}")
    migration = (directory / "000016_jwks_single_active_guard.up.sql").read_text(encoding="utf-8")
    for token in ("active_guard", "uk_jwks_keys_single_active"):
        if token not in migration:
            fail(f"migration 000016 is missing {token}")
    phone = (directory / "000017_users_active_phone_unique_guard.up.sql").read_text(encoding="utf-8")
    for token in ("active_phone", "uk_users_active_phone"):
        if token not in phone:
            fail(f"migration 000017 is missing {token}")
    revocation = (
        directory / "000018_identity_session_revocation_outbox.up.sql"
    ).read_text(encoding="utf-8")
    for token in (
        "identity_session_revocation_outbox",
        "user_version",
        "next_attempt_at",
    ):
        if token not in revocation:
            fail(f"migration 000018 is missing {token}")


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
    identity = (
        ROOT / "internal/apiserver/container/identity/module.go"
    ).read_text(encoding="utf-8")
    for token in ("sessionrevocation.NewWorker", "worker.Run", "IdentityModule) Cleanup"):
        if token not in identity:
            fail(f"identity session revocation worker wiring is missing {token}")


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


def check_readiness_facts() -> None:
    routes = (
        ROOT / "internal/apiserver/transport/rest/base_routes.go"
    ).read_text(encoding="utf-8")
    generic_server = (
        ROOT / "internal/pkg/server/genericapiserver.go"
    ).read_text(encoding="utf-8")
    shutdown = (
        ROOT / "internal/apiserver/process/shutdown_lifecycle.go"
    ).read_text(encoding="utf-8")
    server_check = (
        ROOT / ".github/workflows/server-check.yml"
    ).read_text(encoding="utf-8")
    health_doc = (
        ROOT / "docs/01-运行时/06-健康检查与降级启动.md"
    ).read_text(encoding="utf-8")
    metric_sources = "\n".join(
        path.read_text(encoding="utf-8")
        for path in (
            ROOT / "internal/pkg/middleware/authn/metrics.go",
            ROOT / "internal/apiserver/application/authn/challenge/metrics.go",
            ROOT / "internal/apiserver/transport/grpc/service/authz/metrics.go",
            ROOT / "internal/apiserver/infra/mysql/sessionrevocation/metrics.go",
            ROOT / "internal/apiserver/infra/observability/readiness/metrics.go",
            ROOT / "internal/apiserver/infra/suggest/metrics/metrics.go",
        )
    )

    for token in ('engine.GET("/health",', 'engine.GET("/readyz",'):
        if token not in routes:
            fail(f"REST health route wiring is missing {token}")
    if 'GET("/healthz"' not in generic_server:
        fail("generic server no longer exposes the liveness-only /healthz route")
    for token in ("MarkDraining", "HealthCheckResponse_NOT_SERVING", "drainDelay"):
        if token not in shutdown:
            fail(f"graceful drain wiring is missing {token}")
    for token in ("/healthz", "/readyz", "WARNING: IAM is live but not ready"):
        if token not in server_check:
            fail(f"server-check probe contract is missing {token}")
    for fact in (
        "`/healthz`",
        "`/readyz`",
        "不访问 MySQL/Redis",
        "`/readyz` 失败只告警",
        "-> 停止后台任务",
    ):
        if fact not in health_doc:
            fail(f"health documentation is missing current readiness fact: {fact}")
    for metric_token in (
        'Subsystem: "authz_http"',
        'Subsystem: "grpc_assignment"',
        'Subsystem: "authn_otp"',
        'Subsystem: "identity_session_revocation"',
        'Subsystem: "readiness"',
        'Name:      "refresh_total"',
    ):
        if metric_token not in metric_sources:
            fail(f"required low-cardinality IAM metric is missing {metric_token}")


def check_security_delivery_facts() -> None:
    prod = load_yaml("configs/apiserver.prod.yaml")
    expected_logging = {
        "mysql.log-level": prod.get("mysql", {}).get("log-level"),
        "log.format": prod.get("log", {}).get("format"),
        "log.enable-color": prod.get("log", {}).get("enable-color"),
        "log.development": prod.get("log", {}).get("development"),
    }
    if expected_logging != {
        "mysql.log-level": 1,
        "log.format": "json",
        "log.enable-color": False,
        "log.development": False,
    }:
        fail(f"production logging contract drifted: {expected_logging}")

    metadata = (ROOT / "scripts/cd/image-metadata.sh").read_text(encoding="utf-8")
    remote = (ROOT / "scripts/cd/remote-deploy.sh").read_text(encoding="utf-8")
    dockerfile = (ROOT / "build/docker/Dockerfile").read_text(encoding="utf-8")
    compose = (
        ROOT / "build/docker/docker-compose.prod.yml"
    ).read_text(encoding="utf-8")
    for token in ("HEALTH_PATH", "/healthz", "READINESS_PATH", "/readyz"):
        if token not in metadata:
            fail(f"image metadata is missing delivery probe fact {token}")
    for token in (
        '${HEALTH_PATH}',
        '${READINESS_PATH}',
        'iam_probe_gate_allows "$health_status" "$readiness_status"',
    ):
        if token not in remote:
            fail(f"remote deployment is missing delivery gate {token}")
    for source, label in ((dockerfile, "Dockerfile"), (compose, "production compose")):
        if "localhost:9080/healthz" not in source:
            fail(f"{label} no longer uses /healthz for container liveness")
    for token in ("./cmd/iam-maintenance/", "/app/iam-maintenance"):
        if token not in dockerfile:
            fail(f"production image is missing maintenance binary fact {token}")

    refresh_store = (
        ROOT / "internal/apiserver/infra/cache/redis/token-store.go"
    ).read_text(encoding="utf-8")
    refresher = (
        ROOT / "internal/apiserver/application/authn/token/refresher.go"
    ).read_text(encoding="utf-8")
    profile_link = (
        ROOT
        / "internal/apiserver/transport/grpc/service/identity/profile_link_command.go"
    ).read_text(encoding="utf-8")
    for source, tokens, label in (
        (
            refresh_store,
            ('log.String("key", key)', 'log.String("error", err.Error())'),
            "refresh token Redis logging",
        ),
        (refresher, ("token_hint", "MaskToken"), "refresh conflict logging"),
        (profile_link, ("Error: err.Error()",), "batch gRPC error response"),
    ):
        for token in tokens:
            if token in source:
                fail(f"{label} contains retired unsafe token {token}")

    mapper = (ROOT / "internal/pkg/grpc/error_mapper.go").read_text(
        encoding="utf-8"
    )
    for token in (
        'return "internal server error"',
        'return "service unavailable"',
        "func PublicStatusMessage",
    ):
        if token not in mapper:
            fail(f"gRPC public error mapper is missing {token}")

    operations_doc = (
        ROOT / "docs/01-运行时/07-安全日志与凭据处置.md"
    ).read_text(encoding="utf-8")
    for fact in (
        "默认 dry-run",
        "PURGE_REFRESH_TOKENS",
        "DELETE_PRE_5_4_IAM_LOGS",
        "至少观察一个 Access Token TTL",
    ):
        if fact not in operations_doc:
            fail(f"security operations documentation is missing {fact}")


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
        for stale_identity_fact in (
            "数据库没有手机号唯一索引",
            "`Deactivate` 不会",
            "Identity 已提交，Session 撤销失败 | User 仍 blocked，调用方收到错误",
        ):
            if stale_identity_fact in text:
                fail(f"{path.relative_to(ROOT)} contains stale identity fact: {stale_identity_fact}")

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
        check_readiness_facts,
        check_security_delivery_facts,
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
