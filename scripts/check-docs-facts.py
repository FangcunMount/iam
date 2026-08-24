#!/usr/bin/env python3
"""Check repository facts that active documentation relies on."""

from __future__ import annotations

import json
from pathlib import Path
import re
import subprocess
import sys

import yaml


ROOT = Path(__file__).resolve().parent.parent
ACTIVE_STATUS_PATTERN = re.compile(
    r"^> 状态：(已实现|规划改造)(?: · .+)?$", re.MULTILINE
)
PERSONAL_GO_PATH_PATTERN = re.compile(
    r"/Users/[^/\s]+/\.gvm/gos/[^/\s]+/bin/go"
)


def fail(message: str) -> None:
    raise AssertionError(message)


def load_yaml(relative: str) -> dict:
    with (ROOT / relative).open(encoding="utf-8") as handle:
        return yaml.safe_load(handle) or {}


def active_markdown_files() -> list[Path]:
    return sorted(
        path
        for path in (ROOT / "docs").rglob("*.md")
        if "_archive" not in path.parts
    )


def proto_services(path: Path) -> list[str]:
    return re.findall(
        r"^\s*service\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{",
        path.read_text(encoding="utf-8"),
        re.MULTILINE,
    )


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


def check_generated_document_facts() -> None:
    api_readme = (ROOT / "api/README.md").read_text(encoding="utf-8")
    documented_services = {
        f"api/{href}": re.findall(r"`([A-Za-z_][A-Za-z0-9_]*)`", cell)
        for href, cell in re.findall(
            r"^\|\s*\[[^\]]+\]\((grpc/iam/[^)]+\.proto)\)\s*\|\s*(.*?)\s*\|\s*$",
            api_readme,
            re.MULTILINE,
        )
    }
    contract_services = {}
    for path in sorted((ROOT / "api/grpc/iam").rglob("*.proto")):
        services = proto_services(path)
        if services:
            contract_services[str(path.relative_to(ROOT))] = services
    if documented_services.keys() != contract_services.keys():
        fail(
            "api/README.md gRPC service matrix contracts differ from service-bearing "
            f"proto files: documented={sorted(documented_services)} "
            f"actual={sorted(contract_services)}"
        )
    for relative, services in contract_services.items():
        if documented_services[relative] != services:
            fail(
                f"api/README.md gRPC services for {relative} differ: "
                f"documented={documented_services[relative]} actual={services}"
            )

    rest_readme = (ROOT / "api/rest/README.md").read_text(encoding="utf-8")
    authorization_examples = []
    for raw_payload in re.findall(r"-d\s+'(\{[^'\n]+\})'", rest_readme):
        try:
            payload = json.loads(raw_payload)
        except json.JSONDecodeError as error:
            fail(f"api/rest/README.md contains invalid JSON example: {error}")
        if "resource" in payload or "action" in payload:
            authorization_examples.append(payload)
    if len(authorization_examples) != 1:
        fail(
            "api/rest/README.md must contain exactly one resource/action "
            f"authorization example, found {len(authorization_examples)}"
        )
    example = authorization_examples[0]
    resource = example.get("resource")
    action = example.get("action")
    if not isinstance(resource, str) or not isinstance(action, str):
        fail("api/rest/README.md authorization example needs string resource and action")
    bootstrap = (ROOT / "configs/mysql/bootstrap.sql").read_text(encoding="utf-8")
    resource_offset = bootstrap.find(f"'{resource}'")
    if resource_offset < 0:
        fail(f"documented authorization resource is absent from bootstrap.sql: {resource}")
    action_array = re.search(
        r"JSON_ARRAY\(([^)]*)\)",
        bootstrap[resource_offset : resource_offset + 2048],
        re.DOTALL,
    )
    if action_array is None:
        fail(f"bootstrap.sql resource has no action array: {resource}")
    actions = re.findall(r"'([^']+)'", action_array.group(1))
    if action not in actions:
        fail(
            f"documented authorization action {action} is invalid for {resource}: "
            f"actual={actions}"
        )

    dev = load_yaml("configs/apiserver.dev.yaml")
    dev_port = dev.get("insecure", {}).get("bind-port")
    if not isinstance(dev_port, int):
        fail("configs/apiserver.dev.yaml insecure.bind-port must be an integer")
    root_readme = (ROOT / "README.md").read_text(encoding="utf-8")
    quick_start = re.search(
        r"^### 健康检查\s+(.*?)(?=^### |\Z)",
        root_readme,
        re.MULTILINE | re.DOTALL,
    )
    if quick_start is None:
        fail("README.md is missing the Quick Start health-check section")
    quick_start_ports = re.findall(
        r"http://localhost:(\d+)/", quick_start.group(1)
    )
    if not quick_start_ports or set(quick_start_ports) != {str(dev_port)}:
        fail(
            "README.md Quick Start ports differ from "
            f"configs/apiserver.dev.yaml: documented={quick_start_ports} actual={dev_port}"
        )

    status_counts = {"已实现": 0, "规划改造": 0}
    active_docs = active_markdown_files()
    for path in active_docs:
        text = path.read_text(encoding="utf-8")
        statuses = ACTIVE_STATUS_PATTERN.findall(text)
        if len(statuses) == 1:
            status_counts[statuses[0]] += 1
        if PERSONAL_GO_PATH_PATTERN.search(text):
            fail(
                f"{path.relative_to(ROOT)} contains a personal Go executable path; "
                "use go test or a repository Make target"
            )
        if "提示词" in path.name:
            fail(f"historical prompt remains in active docs: {path.relative_to(ROOT)}")
    status_summary = (
        f"Active docs 状态计数：总计 `{len(active_docs)}` 篇，"
        f"`已实现` `{status_counts['已实现']}` 篇，"
        f"`规划改造` `{status_counts['规划改造']}` 篇。"
    )
    acceptance = (
        ROOT / "docs/01-运行时/08-IAM重构最终验收记录.md"
    ).read_text(encoding="utf-8")
    if status_summary not in acceptance:
        fail(
            "final acceptance record has a stale active-doc status count; "
            f"expected: {status_summary}"
        )
    archived_prompt = (
        ROOT / "docs/_archive/2026-08-18-遗留资产安全退役目标提示词.md"
    )
    if not archived_prompt.exists():
        fail("historical legacy-retirement goal prompt is missing from docs/_archive")


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
    if not up or max(up) != 25:
        fail(f"documented latest migration is 25, repository has {max(up) if up else 'none'}")
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
    retirement = (directory / "000019_retire_legacy_tables.up.sql").read_text(
        encoding="utf-8"
    )
    for token in (
        "INSERT IGNORE INTO profiles",
        "INSERT IGNORE INTO profile_links",
        "iam_children_mismatches",
        "iam_guardianship_mismatches",
        "iam_retirement_dependencies",
        "DROP TABLE IF EXISTS",
        "children",
        "guardianships",
    ):
        if token not in retirement:
            fail(f"migration 000019 is missing {token}")
    for forbidden in (
        "auth_accounts",
        "auth_credentials_legacy",
        "schema_version",
        "tenants",
        "data_dictionary",
        "operation_logs",
        "audit_logs",
        "auth_token_audit",
    ):
        if forbidden in retirement:
            fail(f"migration 000019 includes out-of-scope table {forbidden}")
    schema_version_retirement = (
        directory / "000020_retire_schema_version.up.sql"
    ).read_text(encoding="utf-8")
    for token in (
        "iam_schema_version_retirement_assertion",
        "DROP TABLE IF EXISTS schema_version",
    ):
        if token not in schema_version_retirement:
            fail(f"migration 000020 is missing {token}")
    platform_retirement = (
        directory / "000021_retire_unused_platform_tables.up.sql"
    ).read_text(encoding="utf-8")
    for token in (
        "iam_platform_retirement_assertion",
        "DROP TABLE IF EXISTS tenants, data_dictionary",
    ):
        if token not in platform_retirement:
            fail(f"migration 000021 is missing {token}")
    audit_retirement = (
        directory / "000022_retire_unused_audit_tables.up.sql"
    ).read_text(encoding="utf-8")
    for token in (
        "iam_audit_retirement_assertion",
        "DROP TABLE IF EXISTS operation_logs, audit_logs, auth_token_audit",
    ):
        if token not in audit_retirement:
            fail(f"migration 000022 is missing {token}")
    authn_retirement = (
        directory / "000023_retire_legacy_authn_tables.up.sql"
    ).read_text(encoding="utf-8")
    for token in (
        "iam_authn_schema_assertion",
        "iam_authn_data_assertion",
        "iam_authn_dependency_assertion",
        "DROP TABLE IF EXISTS auth_credentials_legacy, auth_accounts",
    ):
        if token not in authn_retirement:
            fail(f"migration 000023 is missing {token}")
    cleanup_retirement = (
        directory / "000024_retire_seeddata_cleanup_tables.up.sql"
    ).read_text(encoding="utf-8")
    for token in (
        "iam_cleanup_schema_assertion",
        "iam_cleanup_data_assertion",
        "iam_cleanup_dependency_assertion",
        "cleanup_bak_perf_testee_profiles_seeddata_dup_20260812_v1",
        "DROP TABLE IF EXISTS",
    ):
        if token not in cleanup_retirement:
            fail(f"migration 000024 is missing {token}")

def check_database_schema_sources() -> None:
    retired_snapshot = ROOT / "configs/mysql/schema.sql"
    if retired_snapshot.exists():
        fail("configs/mysql/schema.sql must not reappear as a second schema source")

    source_note = (ROOT / "configs/mysql/README.md").read_text(encoding="utf-8")
    for token in (
        "schema 唯一事实源",
        "internal/pkg/migration/migrations/*.sql",
        "bootstrap.sql",
        "不能替代迁移",
    ):
        if token not in source_note:
            fail(f"configs/mysql/README.md is missing schema-source fact {token}")

    bootstrap = (ROOT / "configs/mysql/bootstrap.sql").read_text(encoding="utf-8")
    for forbidden in ("CREATE TABLE", "ALTER TABLE", "DROP TABLE"):
        if re.search(rf"(?im)^\s*{forbidden}\b", bootstrap):
            fail(f"bootstrap.sql contains schema-changing statement {forbidden}")
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
        ROOT / "docs/01-运行时/03-后台任务就绪与优雅关闭.md"
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
    for token in ("/healthz", "/readyz", "WARNING: IAM is live but not ready", "/debug/modules", '"module_states"'):
        if token not in server_check:
            fail(f"server-check probe contract is missing {token}")
    for token in ("/version", "gitCommit", "process_start_time_seconds", "{{.Config.Image}}"):
        if token not in server_check:
            fail(f"server-check runtime provenance contract is missing {token}")
    for fact in (
        "`/healthz`",
        "`/readyz`",
        "MarkDraining",
        "NOT_SERVING",
        "session_revocation",
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

    authz_grpc = (
        ROOT / "internal/apiserver/transport/grpc/service/authz/service.go"
    ).read_text(encoding="utf-8")
    for retired in (
        'status.Errorf(codes.Internal, "enforce:',
        'status.Errorf(codes.Internal, "get authorization snapshot:',
    ):
        if retired in authz_grpc:
            fail(f"AuthZ gRPC still exposes a dynamic Internal error via {retired}")
    if authz_grpc.count("iamgrpc.ToStatusError(err)") < 4:
        fail("AuthZ gRPC mutations and reads no longer share the safe error mapper")

    operations_doc = (
        ROOT / "docs/05-工程质量与运维/04-安全日志与凭据处置.md"
    ).read_text(encoding="utf-8")
    for fact in (
        "默认 dry-run",
        "PURGE_REFRESH_TOKENS",
        "DELETE_PRE_5_4_IAM_LOGS",
        "至少观察一个 Access Token TTL",
    ):
        if fact not in operations_doc:
            fail(f"security operations documentation is missing {fact}")


def check_database_operations_facts() -> None:
    workflow = (ROOT / ".github/workflows/db-ops.yml").read_text(encoding="utf-8")
    integration = (ROOT / ".github/workflows/concurrency-tests.yml").read_text(
        encoding="utf-8"
    )
    script = (ROOT / "scripts/dbops/database-operation.sh").read_text(
        encoding="utf-8"
    )
    operations_doc = (ROOT / ".github/workflows/README.md").read_text(
        encoding="utf-8"
    )

    if workflow.count("script_path: scripts/dbops/database-operation.sh") != 7:
        fail("database workflow no longer routes all operations through the repository script")
    for token in (
        "IAM_DB_OPS_ALLOW_DOCKER_CLIENT",
        "performance-schema-status",
        "rolebinding-guard-preflight",
        "rolebinding-deduplicate-dry-run",
        "rolebinding-deduplicate-apply",
    ):
        if token not in workflow:
            fail(f"database operations workflow is missing {token}")
    for forbidden in (
        "apt-get",
        "apk add",
        "script: |",
        "SHOW TABLES",
        "legacy-retirement-preflight",
        "retire-identity",
        "reconcile-authn",
    ):
        if forbidden in workflow:
            fail(f"database workflow contains retired inline behavior {forbidden}")
    for token in (
        "--single-transaction",
        "--quick",
        "--routines",
        "--triggers",
        "--events",
        "--no-tablespaces",
        "--set-gtid-purged=OFF",
        "gzip -t",
        "chmod 0700",
        "chmod 0600",
        "Ver 8\\.",
        "IAM_DB_OPS_ALLOW_DOCKER_CLIENT",
        "mysql:8.0",
        "retired_tables_present=",
        "expected_version=25",
        "performance schema capability:",
        "sys_table_statistics_select=",
        "rds_table_statistics_enabled=",
        "rds_table_statistics_select=",
        "endpoint_provider=",
        "configure_provider_or_server_startup",
    ):
        if token not in script:
            fail(f"database operation script is missing safety contract {token}")
    for token in (
        "Verify database backup, restore, and status with MySQL 8",
        "Run retirement migrations and full-chain tests",
        "TestRetireUnusedPlatformTablesMigrationMySQL",
        "TestRetireUnusedAuditTablesMigrationMySQL",
        "TestRetireLegacyAuthNTablesMigrationMySQL",
        "TestRetireSeeddataCleanupTablesMigrationMySQL",
        "TestFullMigrationChainAndBootstrapMySQL",
        "IAM_DB_OPS_OPERATION=backup",
        "IAM_DB_OPS_OPERATION=restore",
        "IAM_DB_OPS_OPERATION=status",
        "SELECT value FROM restore_fixture",
    ):
        if token not in integration:
            fail(f"MySQL 8 synthetic restore gate is missing {token}")
    for fact in (
        "MySQL 8.x",
        "scripts/dbops/database-operation.sh",
        "原子改名",
        "只读输出 MySQL 客户端版本",
    ):
        if fact not in operations_doc:
            fail(f"database operations documentation is missing {fact}")


def check_compatibility_retirement_evidence() -> None:
    evidence = load_yaml("docs/05-工程质量与运维/compat-consumer-evidence.yaml")
    if evidence.get("schema_version") != 3:
        fail("compatibility consumer evidence has an unsupported schema version")

    repositories = evidence.get("repositories", {})
    for repository in (
        "iam",
        "qs-server",
        "qs-collection-system",
        "seeddata-runner",
        "qs-operating-system",
    ):
        sha = repositories.get(repository, {}).get("sha", "")
        if not re.fullmatch(r"[0-9a-f]{40}", sha):
            fail(f"compatibility consumer evidence is missing a full SHA for {repository}")

    candidates = evidence.get("candidates", {})
    required_candidates = {
        "rest_profile_link_active",
        "module_status_legacy_booleans",
        "suggest_loader_placeholder_tenant_id",
        "sms_mq_topic_config_and_legacy_publisher",
        "sdk_token_claims_tenant_id",
        "sdk_jwks_stats",
    }
    missing = required_candidates - candidates.keys()
    if missing:
        fail(f"compatibility consumer evidence is missing candidates: {sorted(missing)}")

    rta_scan = evidence.get("rta_scan", {})
    if not re.fullmatch(r"[0-9a-f]{40}", rta_scan.get("iam_sha", "")):
        fail("compatibility evidence is missing the RTA scan IAM SHA")
    classifications = rta_scan.get("classifications", {})
    if classifications.get("internal_runtime", {}).get("new_removal_candidates_found") is not False:
        fail("compatibility evidence no longer records the internal runtime RTA result")
    public_sdk = classifications.get("public_sdk", {})
    if public_sdk.get("deprecation_contract_release") != "v2.0.10":
        fail("compatibility evidence is missing the v2 SDK deprecation release")
    if public_sdk.get("latest_published_tag_at_scan") != "v3.0.0":
        fail("compatibility evidence is missing the published v3 SDK boundary")

    go_mod = (ROOT / "go.mod").read_text(encoding="utf-8")
    verifier_types = (ROOT / "pkg/sdk/auth/verifier/types.go").read_text(encoding="utf-8")
    jwks_types = (ROOT / "pkg/sdk/auth/jwks/types.go").read_text(encoding="utf-8")
    sdk_compile = (ROOT / "pkg/sdk/public_api_compile_test.go").read_text(encoding="utf-8")
    sdk_migration = (ROOT / "pkg/sdk/docs/07-migration-breaking-changes.md").read_text(
        encoding="utf-8"
    )
    swagger = (ROOT / "internal/apiserver/docs/swagger.yaml").read_text(encoding="utf-8")
    for token, source, label in (
        ("module github.com/FangcunMount/iam/v3", go_mod, "v3 Go module path"),
        ("v2.0.10", sdk_migration, "SDK deprecation release"),
        ("免除 Batch C 的最短 30 天等待期", sdk_migration, "SDK owner waiver"),
        ("v3.0.0", sdk_migration, "SDK v3 release"),
        ("REST URL、OpenAPI 版本/component ID 和 gRPC proto package 继续保持 v2", sdk_migration, "wire version boundary"),
    ):
        if token not in source:
            fail(f"SDK v3 retirement evidence is missing {label}: {token}")

    for token, source, label in (
        ("TenantID string", verifier_types, "TokenClaims.TenantID"),
        ("type JWKSStats struct", jwks_types, "JWKSStats"),
        ("authjwks.JWKSStats", sdk_compile, "JWKSStats compile contract"),
        ("claims.TenantID", sdk_compile, "TenantID compile contract"),
    ):
        if token in source:
            fail(f"SDK v3 still contains retired {label}: {token}")

    if "github_com_FangcunMount_iam_v2_" not in swagger:
        fail("OpenAPI no longer preserves the stable REST v2 component prefix")
    if "github_com_FangcunMount_iam_v3_" in swagger:
        fail("Go module v3 leaked into public OpenAPI component identifiers")

    for candidate in (
        "rest_profile_link_active",
        "module_status_legacy_booleans",
        "suggest_loader_placeholder_tenant_id",
        "sms_mq_topic_config_and_legacy_publisher",
    ):
        if candidates.get(candidate, {}).get("status") != "retired_with_owner_waiver":
            fail(f"runtime compatibility candidate is not retired: {candidate}")

    for candidate in ("sdk_token_claims_tenant_id", "sdk_jwks_stats"):
        if candidates.get(candidate, {}).get("status") != "removed_in_v3":
            fail(f"SDK compatibility candidate is not removed in v3: {candidate}")
        migrations = candidates.get(candidate, {}).get("consumer_migration", {})
        for repository, pull_request in (("qs-server", "PR #7"), ("seeddata-runner", "PR #4")):
            migration = migrations.get(repository, "")
            if "v3.0.0" not in migration or pull_request not in migration:
                fail(f"{candidate} is missing the {repository} v3 migration evidence")

    waiver = evidence.get("owner_waiver", {})
    if waiver.get("granted_at") != "2026-08-12":
        fail("Batch C owner waiver is missing its grant date")


def check_active_docs() -> None:
    planning_docs: list[Path] = []
    for path in active_markdown_files():
        text = path.read_text(encoding="utf-8")
        statuses = ACTIVE_STATUS_PATTERN.findall(text)
        if len(statuses) != 1:
            fail(
                f"{path.relative_to(ROOT)} must contain exactly one active status "
                "(已实现 or 规划改造)"
            )
        if statuses[0] == "规划改造":
            planning_docs.append(path)
        if path.name != "CONTRIBUTING-DOCS.md":
            for forbidden in (
                "待补证据",
                "待继续按",
                "仍待逐项",
                "待逐项代码核对",
                "若已实现",
                "若已存在",
                "具体以代码为准",
            ):
                if forbidden in text:
                    fail(
                        f"{path.relative_to(ROOT)} contains unowned review placeholder "
                        f"{forbidden}"
                    )
        if "索引和文件 snapshot 仍保存原始手机号" in text:
            fail(f"{path.relative_to(ROOT)} claims the retired Suggest file snapshot still exists")
        for stale_identity_fact in (
            "数据库没有手机号唯一索引",
            "`Deactivate` 不会",
            "Identity 已提交，Session 撤销失败 | User 仍 blocked，调用方收到错误",
        ):
            if stale_identity_fact in text:
                fail(f"{path.relative_to(ROOT)} contains stale identity fact: {stale_identity_fact}")

    root_status = ACTIVE_STATUS_PATTERN.findall(
        (ROOT / "docs/README.md").read_text(encoding="utf-8")
    )
    if root_status == ["已实现"] and planning_docs:
        names = ", ".join(str(path.relative_to(ROOT)) for path in planning_docs[:3])
        fail(f"docs/README.md cannot be 已实现 while child docs remain 规划改造: {names}")

    acceptance = ROOT / "docs/01-运行时/08-IAM重构最终验收记录.md"
    if not acceptance.exists():
        fail("final IAM refactor acceptance record is missing")
    acceptance_text = acceptance.read_text(encoding="utf-8")
    for required in (
        "最终源码 SHA",
        "镜像 digest",
        "CodeQL",
        "MySQL 8 workflow",
        "Production Deploy",
        "Refresh Token purge",
        "一个 Access Token TTL 观察",
        "不得再次执行 purge 来制造历史证据",
    ):
        if required not in acceptance_text:
            fail(f"final acceptance record is missing safe evidence field {required}")

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
        check_database_schema_sources,
        check_event_catalog,
        check_module_wiring,
        check_jwks_lifecycle_wiring,
        check_readiness_facts,
        check_security_delivery_facts,
        check_database_operations_facts,
        check_compatibility_retirement_evidence,
        check_active_docs,
        check_generated_document_facts,
    )
    try:
        for check in checks:
            check()
        run_contract_check("check-openapi-contracts.py")
        run_contract_check("check-route-contracts.py")
    except (AssertionError, subprocess.CalledProcessError) as error:
        print(f"docs facts failed: {error}", file=sys.stderr)
        return 1
    print(
        "Documentation facts match configuration, migrations, events, routes, "
        "module wiring, and generated documentation facts."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
