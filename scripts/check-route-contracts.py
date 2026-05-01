#!/usr/bin/env python3
"""
Compare REST routes exposed in swagger (generated from code) with OpenAPI specs under api/rest.
Serves as a contract check: what code advertises vs what the spec documents.

Normalization:
- swagger basePath (e.g., /api/v2) + path are concatenated, then we strip a leading "/api"
  to align with OpenAPI paths that start with "/v2/...".
- Path params are already in {param} style in both swagger and OpenAPI.
"""
from pathlib import Path
import sys
import yaml

ROOT = Path(__file__).resolve().parent.parent
SWAGGER_PATH = ROOT / "internal/apiserver/docs/swagger.yaml"
REST_SPECS = [
    ROOT / "api/rest/authn.v2.yaml",
    ROOT / "api/rest/identity.v2.yaml",
    ROOT / "api/rest/authz.v2.yaml",
    ROOT / "api/rest/idp.v2.yaml",
    ROOT / "api/rest/suggest.v2.yaml",
]
IGNORE_ROUTES = {
    "get /v2/authz/health",
    "get /v2/idp/health",
}


def load_yaml(path: Path):
    with path.open("r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def normalize_path(path: str) -> str:
    if not path.startswith("/"):
        path = "/" + path
    # drop leading /api to align with runtime prefixes
    if path.startswith("/api/"):
        path = path[len("/api") :]
    # special-case well-known to be versionless
    if "/.well-known" in path:
        idx = path.index("/.well-known")
        path = path[idx:]
    # remove duplicate slashes
    while "//" in path:
        path = path.replace("//", "/")
    # remove trailing slash (except root)
    if path != "/" and path.endswith("/"):
        path = path[:-1]
    # normalize path params: {foo} -> {}
    out = []
    i = 0
    while i < len(path):
        if path[i] == "{":
            while i < len(path) and path[i] != "}":
                i += 1
            if i < len(path) and path[i] == "}":
                i += 1
            out.append("{}")
        else:
            out.append(path[i])
            i += 1
    path = "".join(out)
    return path


def collect_swagger_routes(swagger: dict) -> set[str]:
    base = swagger.get("basePath", "") or ""
    paths = swagger.get("paths", {})
    routes: set[str] = set()
    verbs = {"get", "post", "put", "patch", "delete", "options", "head"}
    for p, item in paths.items():
        for method in item.keys():
            if method.lower() not in verbs:
                continue
            full = normalize_path(base + p)
            routes.add(f"{method.lower()} {full}")
    return routes


def collect_oas_routes(spec: dict) -> set[str]:
    servers = spec.get("servers") or []
    base = ""
    if servers:
        # take first server url path part
        url = servers[0].get("url", "")
        if "://" in url:
            base = "/" + url.split("://", 1)[1].split("/", 1)[-1]
        else:
            base = url
    routes: set[str] = set()
    verbs = {"get", "post", "put", "patch", "delete", "options", "head"}
    for p, item in spec.get("paths", {}).items():
        for method in item.keys():
            if method.lower() not in verbs:
                continue
            routes.add(f"{method.lower()} {normalize_path(base + p)}")
    return routes


def main() -> int:
    swagger = load_yaml(SWAGGER_PATH)
    swagger_routes = collect_swagger_routes(swagger)

    oas_routes: set[str] = set()
    for spec_path in REST_SPECS:
        oas = load_yaml(spec_path)
        oas_routes |= collect_oas_routes(oas)

    missing_in_code = sorted(oas_routes - swagger_routes - IGNORE_ROUTES)
    undocumented = sorted(swagger_routes - oas_routes - IGNORE_ROUTES)

    if missing_in_code or undocumented:
        if missing_in_code:
            print("Spec routes missing in code (swagger):")
            for r in missing_in_code:
                print(f"  {r}")
        if undocumented:
            print("Code routes undocumented in spec:")
            for r in undocumented:
                print(f"  {r}")
        return 1

    print("Route contracts OK: swagger routes match api/rest specs.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
