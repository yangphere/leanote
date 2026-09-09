"""Fail-closed validator for the DB-independent request input catalog."""

from __future__ import annotations

import json
import sys
from pathlib import Path


def fail(message: str) -> "NoReturn":
    print(f"input contracts invalid: {message}", file=sys.stderr)
    raise SystemExit(1)


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print("usage: validate_input_contracts.py <input-contracts.json> <input-contracts.schema.json>", file=sys.stderr)
        return 2
    try:
        data = json.loads(Path(argv[1]).read_text(encoding="utf-8"))
        schema = json.loads(Path(argv[2]).read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(str(exc))
    if schema.get("$id") != "https://leanote.local/schemas/domain-input-contracts.v1.json":
        fail("unexpected schema artifact")
    if data.get("schema_version") != "leanote.domain-input-contracts.v1":
        fail("unexpected schema_version")
    contracts = data.get("contracts")
    if not isinstance(contracts, list) or len(contracts) != 7:
        fail("expected seven input contracts")
    names = set()
    for contract in contracts:
        name = contract.get("type")
        if not isinstance(name, str) or not name or name in names:
            fail("contract type names must be unique")
        names.add(name)
        fields = contract.get("fields")
        if not isinstance(fields, list) or not fields:
            fail(f"{name} needs fields")
        field_names = set()
        for field in fields:
            for key in ("name", "go_type", "wire_name", "required", "nullable", "notes"):
                if key not in field:
                    fail(f"{name} field missing {key}")
            if "required_in" in field:
                if not isinstance(field["required_in"], list) or len(set(field["required_in"])) != len(field["required_in"]):
                    fail(f"{name}.{field['name']} required_in must be a unique list")
            if field["name"] in field_names:
                fail(f"{name} field names must be unique")
            field_names.add(field["name"])
        if not isinstance(contract.get("runtime_unknowns"), list) or not contract["runtime_unknowns"]:
            fail(f"{name} must retain runtime unknowns")
    print(f"input contracts valid: {len(contracts)} types")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
