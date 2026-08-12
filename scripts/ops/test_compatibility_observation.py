from __future__ import annotations

from datetime import datetime, timezone
import importlib.util
from pathlib import Path
import sys
import unittest


MODULE_PATH = Path(__file__).with_name("compatibility_observation.py")
SPEC = importlib.util.spec_from_file_location("compatibility_observation", MODULE_PATH)
assert SPEC and SPEC.loader
observation = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = observation
SPEC.loader.exec_module(observation)


def valid_raw(observed_at: str = "2026-08-12T07:40:15Z") -> str:
    lines = [
        observation.BEGIN_MARKER,
        f"observed_at={observed_at}",
        "runtime_sha=d03dd3687a57753746e4f54f41045380c216dd5f",
        "process_start_time_seconds=1786520350.36",
    ]
    for series in sorted(observation.EXPECTED_SERIES):
        value = 1 if 'view="canonical"' in series else 0
        if 'state="absent"' in series:
            value = 1
        lines.append(f"{series} {value}")
    lines.append(observation.END_MARKER)
    return "\n".join(lines)


def artifact_name(observed: int, process: int, **signals: int) -> str:
    values = {
        "profile_active": 0,
        "profile_both": 0,
        "module_legacy": 0,
        "module_combined": 0,
        "config_tenant": 0,
        "config_sms": 0,
        "sms_legacy": 0,
    }
    values.update(signals)
    return observation.artifact_name_for(
        observed,
        process,
        "d03dd3687a57753746e4f54f41045380c216dd5f",
        values,
        observed,
    )


def payload_for(observed_values: list[int], process: int, **signals: int) -> dict:
    return {
        "artifacts": [
            {
                "id": index,
                "expired": False,
                "name": artifact_name(value, process, **signals),
                "workflow_run": {"head_branch": "main", "id": value},
            }
            for index, value in enumerate(observed_values, start=1)
        ]
    }


class SnapshotTests(unittest.TestCase):
    def test_builds_low_cardinality_snapshot_and_artifact_name(self) -> None:
        snapshot = observation.build_snapshot(valid_raw(), 31574930232, 1, "workflow_dispatch")
        self.assertEqual(snapshot["runtime_sha"], "d03dd3687a57753746e4f54f41045380c216dd5f")
        self.assertEqual(snapshot["retirement_signals"]["profile_active"], 0)
        self.assertRegex(snapshot["artifact_name"], observation.ARTIFACT_PATTERN)

    def test_fails_closed_when_a_metric_series_is_missing(self) -> None:
        raw = valid_raw().replace(
            'iam_identity_profile_link_query_total{mode="legacy_active"} 0\n', ""
        )
        with self.assertRaisesRegex(observation.EvidenceError, "catalog mismatch"):
            observation.build_snapshot(raw, 1, 1, "schedule")

    def test_fails_closed_on_unrecognized_snapshot_lines(self) -> None:
        raw = valid_raw().replace(
            observation.END_MARKER,
            "unknown_evidence=1\n" + observation.END_MARKER,
        )
        with self.assertRaisesRegex(observation.EvidenceError, "unrecognized"):
            observation.build_snapshot(raw, 1, 1, "schedule")


class WindowTests(unittest.TestCase):
    def setUp(self) -> None:
        self.process = int(datetime(2026, 8, 1, tzinfo=timezone.utc).timestamp())
        self.minimum = 30 * 24 * 60 * 60
        self.gap = 90 * 60

    def test_thirty_day_contiguous_zero_window_is_ready(self) -> None:
        observations = [self.process + offset * 60 * 60 for offset in range(31 * 24 + 1)]
        report = observation.evaluate_window(
            payload_for(observations, self.process), observations[-1], self.minimum, self.gap
        )
        self.assertTrue(report["ready"])
        self.assertEqual(report["current_segment"]["snapshot_count"], len(observations))

    def test_window_is_not_ready_before_thirty_days(self) -> None:
        observations = [self.process + offset * 60 * 60 for offset in range(29 * 24 + 1)]
        report = observation.evaluate_window(
            payload_for(observations, self.process), observations[-1], self.minimum, self.gap
        )
        self.assertFalse(report["ready"])
        self.assertIn("minimum_observation_window_not_reached", report["reasons"])

    def test_old_process_does_not_replace_thirty_days_of_observations(self) -> None:
        first = self.process + self.gap
        observations = [first + offset * 60 * 60 for offset in range(30 * 24)]
        report = observation.evaluate_window(
            payload_for(observations, self.process), observations[-1], self.minimum, self.gap
        )
        self.assertFalse(report["ready"])
        self.assertIn("minimum_observation_window_not_reached", report["reasons"])

    def test_restart_resets_the_current_window(self) -> None:
        old = [self.process + offset * 60 * 60 for offset in range(31 * 24 + 1)]
        new_process = old[-1] + 60
        recent = [new_process, new_process + 60 * 60]
        artifacts = payload_for(old, self.process)["artifacts"]
        artifacts.extend(payload_for(recent, new_process)["artifacts"])
        report = observation.evaluate_window(
            {"artifacts": artifacts}, recent[-1], self.minimum, self.gap
        )
        self.assertFalse(report["ready"])
        self.assertEqual(report["current_segment"]["process_start_epoch"], new_process)
        self.assertEqual(report["current_segment"]["snapshot_count"], 2)

    def test_legacy_hit_blocks_only_its_runtime_candidate(self) -> None:
        observations = [self.process + offset * 60 * 60 for offset in range(31 * 24 + 1)]
        report = observation.evaluate_window(
            payload_for(observations, self.process, profile_active=1),
            observations[-1],
            self.minimum,
            self.gap,
        )
        self.assertFalse(report["ready"])
        self.assertFalse(report["candidates"]["rest_profile_link_active"]["ready"])
        self.assertTrue(report["candidates"]["module_status_legacy_booleans"]["ready"])

    def test_stale_latest_snapshot_fails_closed(self) -> None:
        observations = [self.process + offset * 60 * 60 for offset in range(31 * 24 + 1)]
        report = observation.evaluate_window(
            payload_for(observations, self.process),
            observations[-1] + self.gap + 1,
            self.minimum,
            self.gap,
        )
        self.assertFalse(report["ready"])
        self.assertIn("latest_snapshot_is_stale", report["reasons"])

    def test_gap_starts_a_new_contiguous_window(self) -> None:
        old = [self.process + offset * 60 * 60 for offset in range(31 * 24 + 1)]
        recent = [old[-1] + self.gap + 1, old[-1] + self.gap + 1 + 60 * 60]
        artifacts = payload_for(old, self.process)["artifacts"]
        artifacts.extend(payload_for(recent, self.process)["artifacts"])
        report = observation.evaluate_window(
            {"artifacts": artifacts}, recent[-1], self.minimum, self.gap
        )
        self.assertFalse(report["ready"])
        self.assertEqual(report["current_segment"]["snapshot_count"], 2)

    def test_non_main_artifacts_do_not_enter_the_window(self) -> None:
        observations = [self.process + offset * 60 * 60 for offset in range(31 * 24 + 1)]
        payload = payload_for(observations, self.process)
        for artifact in payload["artifacts"]:
            artifact["workflow_run"]["head_branch"] = "codex/test"
        report = observation.evaluate_window(
            payload, observations[-1], self.minimum, self.gap
        )
        self.assertFalse(report["ready"])
        self.assertIn("no_valid_compatibility_observation_artifacts", report["reasons"])

    def test_artifact_run_id_must_match_workflow_run(self) -> None:
        observations = [self.process + offset * 60 * 60 for offset in range(31 * 24 + 1)]
        payload = payload_for(observations, self.process)
        for artifact in payload["artifacts"]:
            artifact["workflow_run"]["id"] += 1
        report = observation.evaluate_window(
            payload, observations[-1], self.minimum, self.gap
        )
        self.assertFalse(report["ready"])
        self.assertIn("no_valid_compatibility_observation_artifacts", report["reasons"])


if __name__ == "__main__":
    unittest.main()
