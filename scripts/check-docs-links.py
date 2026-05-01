#!/usr/bin/env python3
"""Check active IAM documentation links and retired fact references."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlparse


DEFAULT_PATHS = [
    "README.md",
    "docs",
    "api",
    "pkg/sdk/README.md",
    "pkg/sdk/docs",
]

SKIP_DIRS = {
    ".git",
    "node_modules",
    "vendor",
    "_archive",
}

RETIRED_REFERENCES = [
    "internal/apiserver/interface",
    "internal/apiserver/routers.go",
    "internal/apiserver/server.go",
    "internal/apiserver/application/uc/ref",
    "internal/apiserver/domain/uc/ref",
    "internal/apiserver/application/authz/assignment",
    "internal/apiserver/domain/authz/assignment",
    "internal/apiserver/infra/jwt",
    "internal/apiserver/domain/authn/token",
    "internal/apiserver/domain/authn/jwks",
    "internal/apiserver/infra/cache/catalog",
    "internal/apiserver/infra/messaging/version_notifier.go",
    "docs/architecture-overview.md",
    "docs/uc-architecture.md",
    "docs/DEPLOYMENT.md",
    "docs/JENKINS_QUICKSTART.md",
    "/api/v2/auth/login",
    "/api/v2/auth/refresh",
    "/api/v2/identity/refs",
    "/api/v2/identity/profiles/register",
    "RefQuery",
    "RefCommand",
    "IsRef",
    "AddRef",
    "ListRefs",
]

INLINE_LINK_RE = re.compile(r"(?<!!)\[[^\]]+\]\(([^)\n]+)\)")
REFERENCE_LINK_RE = re.compile(r"^\s*\[[^\]]+\]:\s+(\S+)", re.MULTILINE)


def active_markdown_files(root: Path, inputs: list[str]) -> list[Path]:
    files: set[Path] = set()
    for item in inputs:
        path = (root / item).resolve()
        if not path.exists():
            continue
        if path.is_file() and path.suffix.lower() == ".md" and not is_skipped(path):
            files.add(path)
        elif path.is_dir():
            for md in path.rglob("*.md"):
                if not is_skipped(md):
                    files.add(md.resolve())
    return sorted(files)


def is_skipped(path: Path) -> bool:
    return any(part in SKIP_DIRS for part in path.parts)


def is_external(dest: str) -> bool:
    if dest.startswith("#"):
        return True
    parsed = urlparse(dest)
    return parsed.scheme in {"http", "https", "mailto", "tel", "data", "javascript"}


def normalize_destination(raw: str) -> str:
    dest = raw.strip()
    if dest.startswith("<") and dest.endswith(">"):
        dest = dest[1:-1].strip()
    if " " in dest:
        before_title = re.split(r'\s+"|\s+\'|\s+\(', dest, maxsplit=1)[0]
        if before_title:
            dest = before_title
    return unquote(dest)


def target_exists(source: Path, dest: str) -> bool:
    local = dest.split("#", 1)[0]
    if not local:
        return True
    if Path(local).is_absolute():
        return True
    target = (source.parent / local).resolve()
    if target.exists():
        return True
    if local.endswith("/") and (target / "README.md").exists():
        return True
    return False


def collect_link_issues(files: list[Path], root: Path) -> list[str]:
    issues: list[str] = []
    for file_path in files:
        text = file_path.read_text(encoding="utf-8")
        links = [m.group(1) for m in INLINE_LINK_RE.finditer(text)]
        links.extend(m.group(1) for m in REFERENCE_LINK_RE.finditer(text))
        for raw in links:
            dest = normalize_destination(raw)
            if is_external(dest):
                continue
            if not target_exists(file_path, dest):
                rel_file = file_path.relative_to(root)
                issues.append(f"{rel_file}: broken link -> {raw}")
    return issues


def collect_retired_reference_issues(files: list[Path], root: Path) -> list[str]:
    issues: list[str] = []
    for file_path in files:
        text = file_path.read_text(encoding="utf-8")
        for lineno, line in enumerate(text.splitlines(), start=1):
            for token in RETIRED_REFERENCES:
                if token in line:
                    rel_file = file_path.relative_to(root)
                    issues.append(f"{rel_file}:{lineno}: retired reference -> {token}")
    return issues


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("paths", nargs="*", default=DEFAULT_PATHS)
    args = parser.parse_args()

    root = Path.cwd().resolve()
    files = active_markdown_files(root, args.paths)
    issues = []
    issues.extend(collect_link_issues(files, root))
    issues.extend(collect_retired_reference_issues(files, root))

    if issues:
        print("Documentation hygiene failed:")
        for issue in issues:
            print(f"- {issue}")
        return 1

    print(f"Documentation hygiene passed: {len(files)} active Markdown files checked.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
