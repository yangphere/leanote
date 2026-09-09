"""Validate the domain model catalog without third-party dependencies.

This is intentionally a small, fail-closed validator for the v1 artifact. It
checks the invariants that cannot be expressed portably by the repository's
standard tooling (counts and unique type names) in addition to the required
shape declared by model-catalog.schema.json.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path


EXPECTED_SCHEMA = "leanote.domain-model-catalog.v1"
DECIDED = {"D-01", "D-02", "D-03", "D-04", "D-06"}


def fail(message: str) -> "NoReturn":
    print(f"model catalog invalid: {message}", file=sys.stderr)
    raise SystemExit(1)


def require(mapping: dict, key: str, context: str) -> object:
    if key not in mapping:
        fail(f"{context} missing {key}")
    return mapping[key]


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print(
            "usage: validate_model_catalog.py <model-catalog.json> <model-catalog.schema.json>",
            file=sys.stderr,
        )
        return 2

    catalog_path, schema_path = map(Path, argv[1:])
    try:
        catalog = json.loads(catalog_path.read_text(encoding="utf-8"))
        schema = json.loads(schema_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        fail(str(exc))

    if not isinstance(catalog, dict):
        fail("top-level catalog must be an object")
    if not isinstance(schema, dict) or schema.get("$id") != "https://leanote.local/schemas/domain-model-catalog.v1.json":
        fail("unexpected schema artifact")
    if catalog.get("schema_version") != EXPECTED_SCHEMA:
        fail(f"schema_version must be {EXPECTED_SCHEMA}")

    counts = require(catalog, "counts", "catalog")
    if not isinstance(counts, dict):
        fail("counts must be an object")
    if counts.get("active_types") != 69:
        fail("counts.active_types must be 69")
    if counts.get("commented_declarations") != 3:
        fail("counts.commented_declarations must be 3")
    if not isinstance(counts.get("business_collections"), int) or counts["business_collections"] < 28:
        fail("counts.business_collections must be an integer >= 28")
    if counts.get("api_actions") != 29:
        fail("counts.api_actions must be 29")

    types = require(catalog, "types", "catalog")
    if not isinstance(types, list):
        fail("types must be an array")
    if len(types) != 69 or any(not isinstance(item, dict) or item.get("status") != "active" for item in types):
        fail("types must contain exactly 69 active entries")
    active = types
    names = [item.get("name") for item in active]
    if any(not isinstance(name, str) or not name for name in names):
        fail("every active type needs a name")
    if len(set(names)) != len(names):
        fail("active type names must be unique")
    for item in active:
        for key in ("primary_role", "secondary_roles", "consumer_status", "consumers", "source", "fields", "fixtures"):
            require(item, key, f"type {item.get('name', '<unknown>')}")
        for field in item["fields"]:
            wire = field.get("wire")
            if not isinstance(wire, dict):
                fail(f"type {item['name']} field {field.get('name')} wire must be an object")
            embedded = wire.get("json_embedded")
            if not isinstance(embedded, bool):
                fail(f"type {item['name']} field {field.get('name')} must declare json_embedded")
            if embedded and (wire.get("json_name") is not None or not isinstance(wire.get("json_embedded_names"), list)):
                fail(f"type {item['name']} embedded field {field.get('name')} needs flattened JSON names")

    commented = require(catalog, "commented_declarations", "catalog")
    if not isinstance(commented, list) or len(commented) != 3:
        fail("commented_declarations must contain exactly 3 entries")

    collections = require(catalog, "collections", "catalog")
    if not isinstance(collections, list) or len(collections) < 28:
        fail("collections must contain at least 28 entries")
    for collection in collections:
        if not isinstance(collection, dict):
            fail("each collection must be an object")
        for key in ("name", "model", "status", "evidence", "unique_indexes"):
            require(collection, key, f"collection {collection.get('name', '<unknown>')}")
        for index in collection["unique_indexes"]:
            if not isinstance(index, dict):
                fail("unique index must be an object")
            for key in ("fields", "status", "evidence"):
                require(index, key, f"index in {collection['name']}")

    api_variants = require(catalog, "api_variants", "catalog")
    if not isinstance(api_variants, list) or len(api_variants) != 29:
        fail("api_variants must contain exactly 29 active API actions")
    routes = set()
    actions = set()
    for variant in api_variants:
        if not isinstance(variant, dict):
            fail("each api variant must be an object")
        for key in ("route", "action", "method", "auth", "request", "success", "failure", "evidence_status", "evidence"):
            require(variant, key, f"api variant {variant.get('action', '<unknown>')}")
        if variant["route"] in routes or variant["action"] in actions:
            fail("api routes and actions must be unique")
        routes.add(variant["route"])
        actions.add(variant["action"])
        if not variant["route"].startswith("/api/"):
            fail(f"API route must start with /api/: {variant['route']}")
        if variant["method"] not in {"GET", "POST", "PUT", "DELETE", "PATCH"}:
            fail(f"unsupported method for {variant['action']}")
        if variant["auth"] not in {"required", "not_required", "unknown"}:
            fail(f"unsupported auth state for {variant['action']}")
        if variant["evidence_status"] not in {"confirmed", "partial", "unknown"}:
            fail(f"unsupported evidence status for {variant['action']}")
        if not isinstance(variant["evidence"], list) or not variant["evidence"]:
            fail(f"api variant {variant['action']} evidence must be a non-empty array")

    decisions = require(catalog, "decisions", "catalog")
    if not isinstance(decisions, list):
        fail("decisions must be an array")
    decision_ids = {item.get("id") for item in decisions if isinstance(item, dict)}
    if len(decisions) != len(DECIDED) or decision_ids != DECIDED:
        fail(f"decisions must contain exactly {sorted(DECIDED)}")
    for item in decisions:
        if not isinstance(item, dict) or item.get("status") != "decided":
            fail("every decision must have status=decided")
        for key in ("id", "question", "decision", "impact", "evidence"):
            require(item, key, f"decision {item.get('id', '<unknown>')}")
        if not isinstance(item["evidence"], list) or not item["evidence"]:
            fail(f"decision {item['id']} evidence must be a non-empty array")
    if "D-05" in decision_ids:
        fail("D-05 is a fixed boundary, not a decision entry")

    fixed_boundaries = require(catalog, "fixed_boundaries", "catalog")
    if not isinstance(fixed_boundaries, list) or not any("D-05" in str(item) for item in fixed_boundaries):
        fail("fixed_boundaries must document D-05")

    print(f"model catalog valid: {len(active)} active types, {len(collections)} collections")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
