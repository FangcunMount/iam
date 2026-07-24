#!/usr/bin/env python3
"""Reject new or stale Spectral warnings using stable warning fingerprints."""

from __future__ import annotations

from collections import Counter
import argparse
import json
from pathlib import Path
import sys


def normalized_source(raw: str) -> str:
    marker = "api/rest/"
    normalized = raw.replace("\\", "/")
    if marker in normalized:
        return marker + normalized.split(marker, 1)[1]
    return Path(normalized).name


def fingerprint(item: dict) -> dict:
    return {
        "source": normalized_source(str(item.get("source", ""))),
        "code": str(item.get("code", "")),
        "path": item.get("path", []),
        "message": str(item.get("message", "")),
    }


def encoded(item: dict) -> str:
    return json.dumps(item, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def warning_fingerprints(report: list[dict]) -> list[dict]:
    # Spectral severity 1 is warning; errors (0) are rejected by the lint command.
    return sorted(
        (fingerprint(item) for item in report if item.get("severity") == 1),
        key=encoded,
    )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    parser.add_argument("baseline", type=Path)
    parser.add_argument("--write-baseline", action="store_true")
    args = parser.parse_args()

    report = json.loads(args.report.read_text(encoding="utf-8"))
    current = warning_fingerprints(report)
    if args.write_baseline:
        args.baseline.write_text(
            json.dumps({"version": 1, "warnings": current}, ensure_ascii=False, indent=2)
            + "\n",
            encoding="utf-8",
        )
        print(f"Wrote {len(current)} Spectral warning fingerprints to {args.baseline}")
        return 0

    baseline_doc = json.loads(args.baseline.read_text(encoding="utf-8"))
    expected = baseline_doc.get("warnings", [])
    current_counts = Counter(map(encoded, current))
    expected_counts = Counter(map(encoded, expected))
    new = list((current_counts - expected_counts).elements())
    stale = list((expected_counts - current_counts).elements())
    if new or stale:
        if new:
            print("New Spectral warnings are not allowed:", file=sys.stderr)
            for item in new:
                print(f"  {item}", file=sys.stderr)
        if stale:
            print(
                "Spectral warning baseline is stale; remove resolved warnings:",
                file=sys.stderr,
            )
            for item in stale:
                print(f"  {item}", file=sys.stderr)
        return 1

    print(f"Spectral warning baseline matches ({len(current)} warnings).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
