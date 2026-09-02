#!/usr/bin/env python3
"""Check active IAM documentation links, repo paths, and retired references."""

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
}

RETIRED_REFERENCES = [
    "internal/apiserver/interface",
    "internal/apiserver/routers.go",
    "internal/apiserver/server.go",
    "internal/apiserver/application/identity/ref",
    "internal/apiserver/domain/identity/ref",
    "internal/apiserver/application/authz/rolebinding",
    "internal/apiserver/domain/authz/rolebinding",
    "internal/apiserver/application/authz/assignmentauth",
    "internal/apiserver/application/authz/policysync",
    "internal/apiserver/application/authz/shared",
    "internal/apiserver/infra/authz/native",
    "internal/apiserver/infra/mysql/rolebinding",
    "internal/apiserver/infra/jwt",
    "internal/apiserver/domain/authn/jwks",
    "internal/apiserver/application/authn/token/issuer.go",
    "internal/apiserver/application/authn/token/refresher.go",
    "internal/apiserver/application/authn/token/verifier.go",
    "internal/apiserver/application/authn/token/revoker.go",
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
    "/api/v2/authz",
    "internal/apiserver/domain/authn/session/evaluator.go",
    "domain/authn/session/evaluator.go",
    "internal/apiserver/infra/redis/authn",
    "configs/casbin_model.conf",
    "internal/apiserver/infra/casbin/model.conf",
    "internal/apiserver/modules/authz/infra/casbin/model.conf",
    "internal/apiserver/infra/mysql/casbinrule",
    "internal/apiserver/domain/authz/policy/port/driven/casbin.go",
    "IsSuperAdmin",
    "RefQuery",
    "RefCommand",
    "IsRef",
    "AddRef",
    "ListRefs",
    "SuggestSnapshot",
    "SuggestResult",
    "ProfileSuggestResult",
    "authn-domain-model.png",
    "authn-domain-model-v2",
    "authz-domain-model-v1",
    "core-domain-model.png",
    "core-module-identity-anchor",
    "supporting-domain-model",
]

INLINE_LINK_RE = re.compile(r"(?<!!)\[[^\]]+\]\(([^)\n]+)\)")
IMAGE_LINK_RE = re.compile(r"!\[[^\]]*\]\(([^)\n]+)\)")
REFERENCE_LINK_RE = re.compile(r"^\s*\[[^\]]+\]:\s+(\S+)", re.MULTILINE)
BACKTICK_RE = re.compile(r"`([^`\n]+)`")
REPO_PATH_PREFIXES = (
    ".github/",
    "api/",
    "build/",
    "cmd/",
    "configs/",
    "internal/",
    "pkg/",
    "scripts/",
    "web/",
)
SOURCE_PATH_SUFFIXES = {
    ".go",
    ".md",
    ".proto",
    ".py",
    ".sh",
    ".sql",
    ".conf",
    ".yaml",
    ".yml",
}


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
    if any(part in SKIP_DIRS for part in path.parts):
        return True
    if "_archive" not in path.parts:
        return False
    return not (path.name == "README.md" and path.parent.name == "_archive")


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
        links.extend(m.group(1) for m in IMAGE_LINK_RE.finditer(text))
        links.extend(m.group(1) for m in REFERENCE_LINK_RE.finditer(text))
        for raw in links:
            dest = normalize_destination(raw)
            if is_external(dest):
                continue
            if not target_exists(file_path, dest):
                rel_file = file_path.relative_to(root)
                issues.append(f"{rel_file}: broken link -> {raw}")
    return issues


def normalize_code_path(raw: str) -> str | None:
    candidate = raw.strip().rstrip(".,;:，。；：")
    is_relative = candidate.startswith(("./", "../"))
    is_repo_source_file = (
        candidate.startswith(REPO_PATH_PREFIXES)
        and Path(candidate).suffix in SOURCE_PATH_SUFFIXES
    )
    if not (is_relative or is_repo_source_file):
        return None
    if any(char.isspace() for char in candidate):
        return None
    if any(token in candidate for token in ("*", "{", "}", "<", ">", "$", "|", "...")):
        return None
    return re.sub(r":\d+(?:-\d+)?$", "", candidate)


def collect_code_path_issues(files: list[Path], root: Path) -> list[str]:
    issues: list[str] = []
    for file_path in files:
        text = file_path.read_text(encoding="utf-8")
        for lineno, line in enumerate(text.splitlines(), start=1):
            for raw in BACKTICK_RE.findall(line):
                candidate = normalize_code_path(raw)
                if candidate is None:
                    continue
                target = (
                    file_path.parent / candidate
                    if candidate.startswith(("./", "../"))
                    else root / candidate
                )
                if not target.resolve().exists():
                    rel_file = file_path.relative_to(root)
                    issues.append(f"{rel_file}:{lineno}: stale repo path -> {raw}")
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
    issues.extend(collect_code_path_issues(files, root))
    issues.extend(collect_retired_reference_issues(files, root))

    if issues:
        print("Documentation hygiene failed:")
        for issue in issues:
            print(f"- {issue}")
        return 1

    print(
        f"Documentation hygiene passed: {len(files)} active and archive-index "
        "Markdown files checked."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
