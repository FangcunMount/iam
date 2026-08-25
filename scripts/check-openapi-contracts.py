#!/usr/bin/env python3
"""
Compare swagger2 definitions (internal/apiserver/docs/swagger.yaml)
with api/rest OpenAPI 3.1 component schemas to catch drift.

Usage:
  python scripts/check-openapi-contracts.py
"""
import sys
from pathlib import Path
import yaml

ROOT = Path(__file__).resolve().parent.parent
SWAGGER_PATH = ROOT / "internal/apiserver/docs/swagger.yaml"
REST_SPECS = [
    ROOT / "api/rest/authn.v2.yaml",
    ROOT / "api/rest/identity.v2.yaml",
    ROOT / "api/rest/authz.v3.yaml",
    ROOT / "api/rest/idp.v2.yaml",
]


def load_yaml(path: Path):
    with path.open("r", encoding="utf-8") as f:
        return yaml.safe_load(f)


def compare(sw_definitions: dict, oas_schemas: dict, spec_name: str) -> list[str]:
    """Return a list of human-readable diffs."""
    diffs: list[str] = []
    for full_name, schema_def in sw_definitions.items():
        short = full_name.split(".")[-1]
        if short not in oas_schemas:
            continue

        sw_props = set(schema_def.get("properties", {}).keys())
        oas_props = set(oas_schemas[short].get("properties", {}).keys())

        extra_sw = sorted(sw_props - oas_props)
        extra_oas = sorted(oas_props - sw_props)

        sw_req = set(schema_def.get("required", []))
        oas_req = set(oas_schemas[short].get("required", []))

        if extra_sw or extra_oas or sw_req != oas_req:
            msg_parts = [f"{spec_name}: schema {short}"]
            if extra_sw:
                msg_parts.append(f"missing in OAS: {extra_sw}")
            if extra_oas:
                msg_parts.append(f"extra in OAS: {extra_oas}")
            if sw_req != oas_req:
                msg_parts.append(
                    f"required mismatch swagger={sorted(sw_req)} oas={sorted(oas_req)}"
                )
            diffs.append(" | ".join(msg_parts))
    return diffs


def main() -> int:
    swagger = load_yaml(SWAGGER_PATH)
    sw_defs = swagger.get("definitions", {})

    sw_groups = {
        "authn": {k: v for k, v in sw_defs.items() if "transport_rest_authn" in k},
        "identity": {k: v for k, v in sw_defs.items() if "transport_rest_identity" in k},
        "authz": {k: v for k, v in sw_defs.items() if "transport_rest_authz" in k},
        "idp": {k: v for k, v in sw_defs.items() if "transport_rest_idp" in k},
    }

    diffs: list[str] = []
    for spec_path in REST_SPECS:
        oas = load_yaml(spec_path)
        oas_schemas = oas.get("components", {}).get("schemas", {})
        if "authn" in spec_path.name:
            diffs.extend(compare(sw_groups["authn"], oas_schemas, spec_path.name))
        elif "identity" in spec_path.name:
            diffs.extend(compare(sw_groups["identity"], oas_schemas, spec_path.name))
        elif "authz" in spec_path.name:
            diffs.extend(compare(sw_groups["authz"], oas_schemas, spec_path.name))
        elif "idp" in spec_path.name:
            diffs.extend(compare(sw_groups["idp"], oas_schemas, spec_path.name))

    if diffs:
        for d in diffs:
            print(d)
        return 1

    print("OpenAPI specs match swagger definitions.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
