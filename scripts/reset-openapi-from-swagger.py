#!/usr/bin/env python3
"""
Reset api/rest/*.yaml paths from swagger (internal/apiserver/docs/swagger.yaml),
splitting by path prefix rules.

Usage:
  python scripts/reset-openapi-from-swagger.py [--dry-run]
"""
from __future__ import annotations

import argparse
from pathlib import Path
from typing import Any, Dict, List, Tuple

import yaml


ROOT = Path(__file__).resolve().parent.parent
DEFAULT_SWAGGER = ROOT / "internal/apiserver/docs/swagger.yaml"
SPEC_PATHS = {
    "authn": ROOT / "api/rest/authn.v2.yaml",
    "authz": ROOT / "api/rest/authz.v2.yaml",
    "identity": ROOT / "api/rest/identity.v2.yaml",
    "idp": ROOT / "api/rest/idp.v2.yaml",
    "suggest": ROOT / "api/rest/suggest.v2.yaml",
}


def load_yaml(path: Path) -> Dict[str, Any]:
    with path.open("r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def dump_yaml(path: Path, data: Dict[str, Any]) -> None:
    with path.open("w", encoding="utf-8") as f:
        yaml.safe_dump(
            data,
            f,
            allow_unicode=True,
            sort_keys=False,
            width=120,
        )


def map_path(path: str) -> str:
    if path == "/.well-known/jwks.json":
        return path
    if path.startswith("/admin/jwks/"):
        return "/authn" + path
    if path.startswith("/auth/"):
        return "/authn" + path[5:]
    if path.startswith("/signups/") or path == "/signups":
        return "/authn" + path
    if path.startswith("/accounts/") or path == "/accounts":
        return "/authn" + path
    if path.startswith("/authz/") or path.startswith("/idp/"):
        return path
    if path.startswith("/api/v2/suggest/") or path.startswith("/suggest/"):
        # Suggest 独立模块，移除 /api/v2 前缀
        normalized = path
        if normalized.startswith("/api/v2"):
            normalized = normalized[len("/api/v2") :]
        return normalized
    if path.startswith("/children") or path.startswith("/guardians") or path.startswith("/me") or path.startswith("/users"):
        return "/identity" + path
    return path


def module_for_path(path: str) -> str:
    if path == "/.well-known/jwks.json" or path.startswith("/authn/"):
        return "authn"
    if path.startswith("/authz/"):
        return "authz"
    if path.startswith("/identity/"):
        return "identity"
    if path.startswith("/suggest/") or path.startswith("/api/v2/suggest/"):
        return "suggest"
    if path.startswith("/idp/"):
        return "idp"
    return "unknown"


def rewrite_ref(obj: Any) -> Any:
    if isinstance(obj, dict):
        if "$ref" in obj and isinstance(obj["$ref"], str):
            ref = obj["$ref"]
            if ref.startswith("#/definitions/"):
                name = ref[len("#/definitions/") :]
                obj = {**obj, "$ref": f"#/components/schemas/{name}"}
        return {k: rewrite_ref(v) for k, v in obj.items()}
    if isinstance(obj, list):
        return [rewrite_ref(v) for v in obj]
    return obj


def convert_param(param: Dict[str, Any]) -> Dict[str, Any]:
    out: Dict[str, Any] = {
        "name": param.get("name"),
        "in": param.get("in"),
    }
    if "description" in param:
        out["description"] = param["description"]
    if "required" in param:
        out["required"] = param["required"]
    if "deprecated" in param:
        out["deprecated"] = param["deprecated"]

    schema: Dict[str, Any] = {}
    if "schema" in param:
        schema.update(rewrite_ref(param["schema"]))
    for key in (
        "type",
        "format",
        "items",
        "enum",
        "default",
        "maximum",
        "minimum",
        "maxLength",
        "minLength",
        "pattern",
        "maxItems",
        "minItems",
        "uniqueItems",
        "multipleOf",
    ):
        if key in param:
            schema[key] = rewrite_ref(param[key])

    if schema:
        out["schema"] = schema
    return out


def convert_headers(headers: Dict[str, Any]) -> Dict[str, Any]:
    out: Dict[str, Any] = {}
    for name, hdr in headers.items():
        schema = {}
        if "schema" in hdr:
            schema.update(rewrite_ref(hdr["schema"]))
        for key in ("type", "format", "items", "enum", "default"):
            if key in hdr:
                schema[key] = hdr[key]
        out[name] = {"schema": schema} if schema else {}
        if "description" in hdr:
            out[name]["description"] = hdr["description"]
    return out


def convert_responses(responses: Dict[str, Any], default_mime: str) -> Dict[str, Any]:
    out: Dict[str, Any] = {}
    for code, resp in responses.items():
        item: Dict[str, Any] = {"description": resp.get("description", "")}
        if "headers" in resp:
            item["headers"] = convert_headers(resp["headers"])
        if "schema" in resp:
            item["content"] = {
                default_mime: {
                    "schema": rewrite_ref(resp["schema"]),
                }
            }
        out[str(code)] = item
    return out


def convert_operation(op: Dict[str, Any], global_produces: List[str]) -> Dict[str, Any]:
    out: Dict[str, Any] = {}
    for key in ("tags", "summary", "description", "operationId", "deprecated"):
        if key in op:
            out[key] = op[key]

    produces = op.get("produces") or global_produces or ["application/json"]
    mime = produces[0]

    params = op.get("parameters", [])
    non_body_params = []
    request_body = None
    for param in params:
        if param.get("in") == "body":
            schema = rewrite_ref(param.get("schema", {}))
            request_body = {
                "required": param.get("required", False),
                "content": {mime: {"schema": schema}},
            }
        elif param.get("in") == "formData":
            schema = {
                "type": "object",
                "properties": {param["name"]: {"type": param.get("type", "string")}},
            }
            request_body = {
                "required": param.get("required", False),
                "content": {mime: {"schema": schema}},
            }
        else:
            non_body_params.append(convert_param(param))

    if non_body_params:
        out["parameters"] = non_body_params
    if request_body:
        out["requestBody"] = request_body

    out["responses"] = convert_responses(op.get("responses", {}), mime)
    return out


def convert_path_item(item: Dict[str, Any], global_produces: List[str]) -> Dict[str, Any]:
    out: Dict[str, Any] = {}
    if "parameters" in item:
        out["parameters"] = [convert_param(p) for p in item["parameters"]]
    for method in ("get", "post", "put", "patch", "delete", "options", "head"):
        if method in item:
            out[method] = convert_operation(item[method], global_produces)
    return out


def merge_tags(existing: List[Dict[str, Any]], found: List[str]) -> List[Dict[str, Any]]:
    existing_by_name = {t.get("name"): t for t in existing if isinstance(t, dict)}
    for name in found:
        if name not in existing_by_name:
            existing_by_name[name] = {"name": name}
    return list(existing_by_name.values())


def collect_schema_refs(obj: Any, out: set[str]) -> None:
    if isinstance(obj, dict):
        ref = obj.get("$ref")
        if isinstance(ref, str) and ref.startswith("#/components/schemas/"):
            out.add(ref[len("#/components/schemas/") :])
        for v in obj.values():
            collect_schema_refs(v, out)
    elif isinstance(obj, list):
        for v in obj:
            collect_schema_refs(v, out)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--swagger", default=str(DEFAULT_SWAGGER), help="Path to swagger.yaml")
    parser.add_argument("--dry-run", action="store_true", help="Only print summary, do not write files")
    args = parser.parse_args()

    swagger = load_yaml(Path(args.swagger))
    sw_paths = swagger.get("paths", {})
    global_produces = swagger.get("produces", [])

    module_paths: Dict[str, Dict[str, Any]] = {k: {} for k in SPEC_PATHS}
    module_schema_refs: Dict[str, set[str]] = {k: set() for k in SPEC_PATHS}
    unknown_paths: List[str] = []
    all_tags: Dict[str, set] = {k: set() for k in SPEC_PATHS}

    for raw_path, path_item in sw_paths.items():
        mapped = map_path(raw_path)
        module = module_for_path(mapped)
        if module == "unknown":
            unknown_paths.append(raw_path)
            continue
        module_paths[module][mapped] = convert_path_item(path_item, global_produces)
        collect_schema_refs(module_paths[module][mapped], module_schema_refs[module])
        for op in module_paths[module][mapped].values():
            if isinstance(op, dict):
                for tag in op.get("tags", []) or []:
                    all_tags[module].add(tag)

    swagger_defs = swagger.get("definitions", {})

    for module, spec_path in SPEC_PATHS.items():
        spec = load_yaml(spec_path)
        spec["paths"] = module_paths[module]
        spec_tags = spec.get("tags", [])
        spec["tags"] = merge_tags(spec_tags, sorted(all_tags[module]))
        schemas: Dict[str, Any] = {}
        missing = []
        for name in sorted(module_schema_refs[module]):
            if name not in swagger_defs:
                missing.append(name)
                continue
            schemas[name] = rewrite_ref(swagger_defs[name])
        if missing:
            print(f"{spec_path.name}: missing swagger definitions: {missing}")
        components = spec.get("components", {})
        components["schemas"] = schemas
        spec["components"] = components
        if args.dry_run:
            print(f"{spec_path.name}: {len(module_paths[module])} paths, {len(schemas)} schemas")
        else:
            dump_yaml(spec_path, spec)

    if unknown_paths:
        print("Unmapped swagger paths:")
        for p in sorted(unknown_paths):
            print(f"  - {p}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
