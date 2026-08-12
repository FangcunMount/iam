#!/usr/bin/env python3
"""Build and evaluate low-cardinality IAM compatibility observation evidence."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from datetime import datetime, timezone
import json
import math
from pathlib import Path
import re
import sys
from typing import Any


BEGIN_MARKER = "IAM_COMPAT_SNAPSHOT_V1_BEGIN"
END_MARKER = "IAM_COMPAT_SNAPSHOT_V1_END"
ARTIFACT_PREFIX = "iam-compat-v1"
REQUIRED_HEAD_BRANCH = "main"

EXPECTED_SERIES = {
    'iam_authn_sms_publisher_selections_total{mode="catalog"}',
    'iam_authn_sms_publisher_selections_total{mode="legacy"}',
    'iam_config_compatibility_key_observations_total{key="sms.mq.topic",state="absent"}',
    'iam_config_compatibility_key_observations_total{key="sms.mq.topic",state="present"}',
    'iam_config_compatibility_key_observations_total{key="suggest.loader_placeholder_tenant_id",state="absent"}',
    'iam_config_compatibility_key_observations_total{key="suggest.loader_placeholder_tenant_id",state="present"}',
    'iam_identity_profile_link_query_total{mode="both"}',
    'iam_identity_profile_link_query_total{mode="default"}',
    'iam_identity_profile_link_query_total{mode="include_revoked"}',
    'iam_identity_profile_link_query_total{mode="legacy_active"}',
    'iam_runtime_debug_modules_requests_total{view="canonical"}',
    'iam_runtime_debug_modules_requests_total{view="combined"}',
    'iam_runtime_debug_modules_requests_total{view="legacy"}',
}

SIGNAL_SERIES = {
    "profile_active": 'iam_identity_profile_link_query_total{mode="legacy_active"}',
    "profile_both": 'iam_identity_profile_link_query_total{mode="both"}',
    "module_legacy": 'iam_runtime_debug_modules_requests_total{view="legacy"}',
    "module_combined": 'iam_runtime_debug_modules_requests_total{view="combined"}',
    "config_tenant": 'iam_config_compatibility_key_observations_total{key="suggest.loader_placeholder_tenant_id",state="present"}',
    "config_sms": 'iam_config_compatibility_key_observations_total{key="sms.mq.topic",state="present"}',
    "sms_legacy": 'iam_authn_sms_publisher_selections_total{mode="legacy"}',
}

CANDIDATE_SIGNALS = {
    "rest_profile_link_active": ("profile_active", "profile_both"),
    "module_status_legacy_booleans": ("module_legacy", "module_combined"),
    "suggest_loader_placeholder_tenant_id": ("config_tenant",),
    "sms_mq_topic_config_and_legacy_publisher": ("config_sms", "sms_legacy"),
}

ARTIFACT_PATTERN = re.compile(
    rf"^{ARTIFACT_PREFIX}"
    r"-t(?P<observed>\d+)"
    r"-p(?P<process>\d+)"
    r"-h(?P<sha>[0-9a-f]{40})"
    r"-a(?P<profile_active>\d+)"
    r"-b(?P<profile_both>\d+)"
    r"-l(?P<module_legacy>\d+)"
    r"-c(?P<module_combined>\d+)"
    r"-x(?P<config_tenant>\d+)"
    r"-q(?P<config_sms>\d+)"
    r"-s(?P<sms_legacy>\d+)"
    r"-r(?P<run_id>\d+)$"
)

METRIC_PATTERN = re.compile(
    r"^(?P<series>iam_(?:authn_sms_publisher_selections_total|"
    r"config_compatibility_key_observations_total|"
    r"identity_profile_link_query_total|"
    r"runtime_debug_modules_requests_total)\{[^}]+\})\s+"
    r"(?P<value>(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)$"
)


class EvidenceError(ValueError):
    """Raised when observation evidence is incomplete or malformed."""


@dataclass(frozen=True)
class ArtifactSnapshot:
    observed_epoch: int
    process_start_epoch: int
    runtime_sha: str
    signals: dict[str, int]
    run_id: int
    artifact_id: int | None = None
    artifact_url: str = ""


def parse_rfc3339(value: str) -> datetime:
    normalized = value.replace("Z", "+00:00")
    parsed = datetime.fromisoformat(normalized)
    if parsed.tzinfo is None:
        raise EvidenceError("observed_at must include a timezone")
    return parsed.astimezone(timezone.utc)


def integer_counter(value: str, label: str) -> int:
    parsed = float(value)
    if not math.isfinite(parsed) or parsed < 0 or not parsed.is_integer():
        raise EvidenceError(f"{label} must be a finite non-negative integer counter")
    return int(parsed)


def extract_snapshot_block(raw: str) -> list[str]:
    blocks: list[list[str]] = []
    current: list[str] | None = None
    for line in raw.splitlines():
        stripped = line.strip()
        if stripped == BEGIN_MARKER:
            if current is not None:
                raise EvidenceError("nested compatibility snapshot marker")
            current = []
            continue
        if stripped == END_MARKER:
            if current is None:
                raise EvidenceError("compatibility snapshot end marker has no beginning")
            blocks.append(current)
            current = None
            continue
        if current is not None:
            current.append(stripped)
    if current is not None:
        raise EvidenceError("compatibility snapshot is missing its end marker")
    if len(blocks) != 1:
        raise EvidenceError(f"expected one compatibility snapshot block, got {len(blocks)}")
    return blocks[0]


def build_snapshot(
    raw: str,
    run_id: int,
    run_attempt: int,
    event: str,
    workflow_head_sha: str,
) -> dict[str, Any]:
    metadata: dict[str, str] = {}
    metrics: dict[str, int] = {}
    for line in extract_snapshot_block(raw):
        match = METRIC_PATTERN.fullmatch(line)
        if match:
            series = match.group("series")
            if series in metrics:
                raise EvidenceError(f"duplicate compatibility metric series: {series}")
            metrics[series] = integer_counter(match.group("value"), series)
            continue
        if re.fullmatch(r"(?:observed_at|runtime_sha|process_start_time_seconds)=.*", line):
            key, value = line.split("=", 1)
            if key in metadata:
                raise EvidenceError(f"duplicate compatibility metadata key: {key}")
            metadata[key] = value
            continue
        raise EvidenceError(f"unrecognized compatibility snapshot line: {line}")

    required_metadata = {"observed_at", "runtime_sha", "process_start_time_seconds"}
    missing_metadata = required_metadata - metadata.keys()
    if missing_metadata:
        raise EvidenceError(f"missing compatibility metadata: {sorted(missing_metadata)}")
    if not re.fullmatch(r"[0-9a-f]{40}", metadata["runtime_sha"]):
        raise EvidenceError("runtime_sha must be a full lowercase Git SHA")
    if not re.fullmatch(r"[0-9a-f]{40}", workflow_head_sha):
        raise EvidenceError("workflow_head_sha must be a full lowercase Git SHA")
    if metadata["runtime_sha"] != workflow_head_sha:
        raise EvidenceError(
            "runtime_sha must equal the main workflow head SHA before evidence is accepted"
        )

    missing_series = EXPECTED_SERIES - metrics.keys()
    unexpected_series = metrics.keys() - EXPECTED_SERIES
    if missing_series or unexpected_series:
        raise EvidenceError(
            "compatibility metric catalog mismatch: "
            f"missing={sorted(missing_series)} unexpected={sorted(unexpected_series)}"
        )

    observed = parse_rfc3339(metadata["observed_at"])
    observed_epoch = int(observed.timestamp())
    process_start = float(metadata["process_start_time_seconds"])
    if not math.isfinite(process_start) or process_start <= 0:
        raise EvidenceError("process_start_time_seconds must be positive and finite")
    process_start_epoch = int(math.floor(process_start))
    if process_start_epoch > observed_epoch + 5:
        raise EvidenceError("process start time is later than observation time")

    signals = {name: metrics[series] for name, series in SIGNAL_SERIES.items()}
    artifact_name = artifact_name_for(
        observed_epoch,
        process_start_epoch,
        metadata["runtime_sha"],
        signals,
        run_id,
    )
    return {
        "schema_version": 1,
        "observed_at": observed.isoformat().replace("+00:00", "Z"),
        "observed_epoch": observed_epoch,
        "process_start_time_seconds": process_start,
        "process_start_epoch": process_start_epoch,
        "runtime_sha": metadata["runtime_sha"],
        "workflow": {
            "run_id": run_id,
            "run_attempt": run_attempt,
            "event": event,
            "head_sha": workflow_head_sha,
        },
        "metrics": metrics,
        "retirement_signals": signals,
        "artifact_name": artifact_name,
    }


def artifact_name_for(
    observed_epoch: int,
    process_start_epoch: int,
    runtime_sha: str,
    signals: dict[str, int],
    run_id: int,
) -> str:
    return (
        f"{ARTIFACT_PREFIX}-t{observed_epoch}-p{process_start_epoch}-h{runtime_sha}"
        f"-a{signals['profile_active']}-b{signals['profile_both']}"
        f"-l{signals['module_legacy']}-c{signals['module_combined']}"
        f"-x{signals['config_tenant']}-q{signals['config_sms']}"
        f"-s{signals['sms_legacy']}-r{run_id}"
    )


def parse_artifact(artifact: dict[str, Any]) -> ArtifactSnapshot | None:
    if artifact.get("expired") is True:
        return None
    workflow_run = artifact.get("workflow_run")
    if not isinstance(workflow_run, dict):
        return None
    if workflow_run.get("head_branch") != REQUIRED_HEAD_BRANCH:
        return None
    match = ARTIFACT_PATTERN.fullmatch(str(artifact.get("name", "")))
    if not match:
        return None
    values = match.groupdict()
    run_id = int(values["run_id"])
    workflow_run_id = workflow_run.get("id")
    if workflow_run_id is not None and int(workflow_run_id) != run_id:
        return None
    workflow_head_sha = workflow_run.get("head_sha")
    if workflow_head_sha != values["sha"]:
        return None
    signal_names = (
        "profile_active",
        "profile_both",
        "module_legacy",
        "module_combined",
        "config_tenant",
        "config_sms",
        "sms_legacy",
    )
    return ArtifactSnapshot(
        observed_epoch=int(values["observed"]),
        process_start_epoch=int(values["process"]),
        runtime_sha=values["sha"],
        signals={name: int(values[name]) for name in signal_names},
        run_id=run_id,
        artifact_id=int(artifact["id"]) if artifact.get("id") is not None else None,
        artifact_url=str(artifact.get("archive_download_url", "")),
    )


def flatten_artifacts(payload: Any) -> list[dict[str, Any]]:
    if isinstance(payload, dict):
        artifacts = payload.get("artifacts", [])
        return [item for item in artifacts if isinstance(item, dict)]
    if not isinstance(payload, list):
        raise EvidenceError("artifact payload must be an object or list")
    flattened: list[dict[str, Any]] = []
    for item in payload:
        if isinstance(item, dict) and "artifacts" in item:
            flattened.extend(
                artifact for artifact in item.get("artifacts", []) if isinstance(artifact, dict)
            )
        elif isinstance(item, dict):
            flattened.append(item)
    return flattened


def latest_contiguous_segment(
    snapshots: list[ArtifactSnapshot], maximum_gap_seconds: int
) -> list[ArtifactSnapshot]:
    if not snapshots:
        return []
    ordered = sorted(snapshots, key=lambda item: (item.observed_epoch, item.run_id))
    latest = ordered[-1]
    segment = [latest]
    for current in reversed(ordered[:-1]):
        next_item = segment[0]
        if current.runtime_sha != latest.runtime_sha:
            break
        if current.process_start_epoch != latest.process_start_epoch:
            break
        if next_item.observed_epoch - current.observed_epoch > maximum_gap_seconds:
            break
        segment.insert(0, current)
    return segment


def evaluate_window(
    payload: Any,
    now_epoch: int,
    minimum_seconds: int,
    maximum_gap_seconds: int,
) -> dict[str, Any]:
    unique_artifacts: list[dict[str, Any]] = []
    seen_names: set[str] = set()
    for item in flatten_artifacts(payload):
        name = str(item.get("name", ""))
        if name in seen_names:
            continue
        seen_names.add(name)
        unique_artifacts.append(item)
    parsed = [parse_artifact(item) for item in unique_artifacts]
    snapshots = [item for item in parsed if item is not None]
    segment = latest_contiguous_segment(snapshots, maximum_gap_seconds)
    if not segment:
        return {
            "schema_version": 1,
            "ready": False,
            "reasons": ["no_valid_compatibility_observation_artifacts"],
            "candidates": {},
        }

    first = segment[0]
    last = segment[-1]
    gaps = [
        right.observed_epoch - left.observed_epoch
        for left, right in zip(segment, segment[1:])
    ]
    max_gap = max(gaps, default=0)
    initial_delay = first.observed_epoch - first.process_start_epoch
    observed_duration = last.observed_epoch - first.observed_epoch
    process_age = last.observed_epoch - first.process_start_epoch
    last_age = now_epoch - last.observed_epoch

    coverage_reasons: list[str] = []
    if initial_delay < -5 or initial_delay > maximum_gap_seconds:
        coverage_reasons.append("first_snapshot_not_close_to_process_start")
    if observed_duration < minimum_seconds:
        coverage_reasons.append("minimum_observation_window_not_reached")
    if max_gap > maximum_gap_seconds:
        coverage_reasons.append("observation_gap_exceeded")
    if last_age < -300 or last_age > maximum_gap_seconds:
        coverage_reasons.append("latest_snapshot_is_stale")

    candidates: dict[str, Any] = {}
    for candidate, signal_names in CANDIDATE_SIGNALS.items():
        maxima = {
            signal: max(snapshot.signals[signal] for snapshot in segment)
            for signal in signal_names
        }
        signal_hits = {name: value for name, value in maxima.items() if value != 0}
        reasons = list(coverage_reasons)
        if signal_hits:
            reasons.append("legacy_compatibility_signal_observed")
        candidates[candidate] = {
            "ready": not reasons,
            "reasons": reasons,
            "signal_maxima": maxima,
        }

    reasons = list(coverage_reasons)
    if any(not value["ready"] for value in candidates.values()) and not coverage_reasons:
        reasons.append("one_or_more_runtime_candidates_have_legacy_use")
    ready = all(value["ready"] for value in candidates.values())
    return {
        "schema_version": 1,
        "ready": ready,
        "reasons": reasons,
        "policy": {
            "minimum_window_seconds": minimum_seconds,
            "maximum_gap_seconds": maximum_gap_seconds,
            "same_runtime_sha_required": True,
            "same_process_start_required": True,
            "workflow_head_sha_equals_runtime_sha_required": True,
        },
        "current_segment": {
            "runtime_sha": last.runtime_sha,
            "process_start_epoch": last.process_start_epoch,
            "first_observed_epoch": first.observed_epoch,
            "last_observed_epoch": last.observed_epoch,
            "process_age_seconds": process_age,
            "observed_duration_seconds": observed_duration,
            "latest_snapshot_age_seconds": last_age,
            "first_snapshot_delay_seconds": initial_delay,
            "maximum_observed_gap_seconds": max_gap,
            "snapshot_count": len(segment),
            "first_run_id": first.run_id,
            "last_run_id": last.run_id,
        },
        "candidates": candidates,
        "external_gates": {
            "public_sdk": {
                "ready": False,
                "reason": "requires_deprecation_and_major_version_evidence",
            }
        },
    }


def render_markdown(report: dict[str, Any]) -> str:
    status = "READY" if report.get("ready") else "NOT READY"
    lines = ["# IAM compatibility retirement readiness", "", f"Overall: **{status}**", ""]
    segment = report.get("current_segment")
    if segment:
        lines.extend(
            [
                f"- Runtime SHA: `{segment['runtime_sha']}`",
                f"- Process start epoch: `{segment['process_start_epoch']}`",
                f"- Snapshots in current contiguous segment: `{segment['snapshot_count']}`",
                f"- Process age seconds: `{segment['process_age_seconds']}`",
                f"- Observed duration seconds: `{segment['observed_duration_seconds']}`",
                f"- Maximum observed gap seconds: `{segment['maximum_observed_gap_seconds']}`",
                f"- Latest snapshot age seconds: `{segment['latest_snapshot_age_seconds']}`",
                "",
            ]
        )
    if report.get("reasons"):
        lines.append("Reasons: " + ", ".join(f"`{reason}`" for reason in report["reasons"]))
        lines.append("")
    lines.extend(["| Candidate | Ready | Signal maxima | Reasons |", "| --- | --- | --- | --- |"])
    for name, candidate in report.get("candidates", {}).items():
        maxima = ", ".join(f"{key}={value}" for key, value in candidate["signal_maxima"].items())
        reasons = ", ".join(candidate["reasons"]) or "none"
        lines.append(f"| `{name}` | `{candidate['ready']}` | `{maxima}` | `{reasons}` |")
    lines.extend(
        [
            "",
            "> Public SDK removal is intentionally excluded from this runtime gate; it requires deprecation and major-version evidence.",
            "",
        ]
    )
    return "\n".join(lines)


def snapshot_command(args: argparse.Namespace) -> int:
    raw = Path(args.input).read_text(encoding="utf-8")
    snapshot = build_snapshot(
        raw,
        args.run_id,
        args.run_attempt,
        args.event,
        args.workflow_head_sha,
    )
    Path(args.output).write_text(
        json.dumps(snapshot, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(snapshot["artifact_name"])
    return 0


def evaluate_command(args: argparse.Namespace) -> int:
    if args.require_ready and args.current_head_branch != REQUIRED_HEAD_BRANCH:
        raise EvidenceError("the retirement gate can only be required from the main branch")
    payload = json.loads(Path(args.artifacts).read_text(encoding="utf-8"))
    if args.current_artifact_name:
        artifacts = flatten_artifacts(payload)
        artifacts.append(
            {
                "name": args.current_artifact_name,
                "expired": False,
                "workflow_run": {
                    "head_branch": args.current_head_branch,
                    "head_sha": args.current_head_sha,
                    "id": args.current_run_id,
                },
            }
        )
        payload = {"artifacts": artifacts}
    now_epoch = int(parse_rfc3339(args.now).timestamp()) if args.now else int(datetime.now(timezone.utc).timestamp())
    report = evaluate_window(
        payload,
        now_epoch,
        args.minimum_days * 24 * 60 * 60,
        args.maximum_gap_minutes * 60,
    )
    Path(args.output).write_text(
        json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    Path(args.markdown_output).write_text(render_markdown(report), encoding="utf-8")
    if args.require_ready and not report["ready"]:
        return 1
    return 0


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)

    snapshot = commands.add_parser("snapshot", help="Build one structured snapshot")
    snapshot.add_argument("--input", required=True)
    snapshot.add_argument("--output", required=True)
    snapshot.add_argument("--run-id", required=True, type=int)
    snapshot.add_argument("--run-attempt", required=True, type=int)
    snapshot.add_argument("--event", required=True)
    snapshot.add_argument("--workflow-head-sha", required=True)
    snapshot.set_defaults(handler=snapshot_command)

    evaluate = commands.add_parser("evaluate", help="Evaluate the latest contiguous window")
    evaluate.add_argument("--artifacts", required=True)
    evaluate.add_argument("--output", required=True)
    evaluate.add_argument("--markdown-output", required=True)
    evaluate.add_argument("--minimum-days", type=int, default=30)
    evaluate.add_argument("--maximum-gap-minutes", type=int, default=90)
    evaluate.add_argument("--now")
    evaluate.add_argument("--current-artifact-name")
    evaluate.add_argument("--current-head-branch", required=True)
    evaluate.add_argument("--current-head-sha", required=True)
    evaluate.add_argument("--current-run-id", required=True, type=int)
    evaluate.add_argument("--require-ready", action="store_true")
    evaluate.set_defaults(handler=evaluate_command)
    return root


def main() -> int:
    try:
        args = parser().parse_args()
        return args.handler(args)
    except (EvidenceError, OSError, json.JSONDecodeError) as error:
        print(f"compatibility observation failed: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
