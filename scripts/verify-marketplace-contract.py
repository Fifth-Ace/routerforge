#!/usr/bin/env python3
import copy
import importlib.util
import json
import re
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MARKETPLACE = ROOT / "marketplace"
BUILDER_PATH = MARKETPLACE / "build_registry.py"
SCHEMA_PATH = MARKETPLACE / "schema" / "manifest.schema.json"
CATALOG_GO = ROOT / "components" / "core" / "catalog.go"
REGISTRY_GO = ROOT / "components" / "core" / "routerforge_registry.go"

def fail(message):
    raise SystemExit("marketplace-contract: " + message)

def load_builder():
    spec = importlib.util.spec_from_file_location("routerforge_marketplace_builder", BUILDER_PATH)
    if spec is None or spec.loader is None:
        fail("cannot load marketplace/build_registry.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

def function_body(source, name):
    match = re.search(r"func\s+" + re.escape(name) + r"\s*\([^)]*\)[^{]*\{", source)
    if not match:
        fail("runtime function missing: " + name)
    start = match.end()
    depth = 1
    i = start
    while i < len(source) and depth:
        if source[i] == "{":
            depth += 1
        elif source[i] == "}":
            depth -= 1
        i += 1
    if depth:
        fail("runtime function is unbalanced: " + name)
    return source[start:i - 1]

def require_tokens(label, source, values):
    missing = [value for value in sorted(values) if ('"' + value + '"') not in source]
    if missing:
        fail(label + " missing runtime values: " + ", ".join(missing))

def expect_reject(builder, obj, filename, needle):
    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / filename
        try:
            builder.validate_manifest(obj, path)
        except ValueError as exc:
            if needle not in str(exc):
                fail("wrong rejection for " + filename + ": " + str(exc))
            return
    fail("invalid fixture was accepted: " + filename)

def main():
    builder = load_builder()
    schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
    catalog_go = CATALOG_GO.read_text(encoding="utf-8")
    registry_go = REGISTRY_GO.read_text(encoding="utf-8")
    runtime = catalog_go + "\n" + registry_go

    # Gate 1: builder constants and JSON schema must stay aligned.
    builder.validate_contract_alignment()

    # Gate 2: every checked-in submission must remain accepted by the builder.
    submissions = sorted((MARKETPLACE / "submissions").glob("*.json"))
    if not submissions:
        fail("no marketplace submissions found")
    for path in submissions:
        builder.validate_manifest(json.loads(path.read_text(encoding="utf-8")), path)

    # Gate 3: positive fixture covers fields that previously drifted.
    valid = {
        "schema_version": 1,
        "id": "contract-fixture",
        "kind": "integration",
        "name": "Contract Fixture",
        "publisher": {"id": "fixture", "name": "Fixture"},
        "source": "project-official",
        "version_source": "opkg",
        "conflicts": ["legacy-fixture"],
        "capabilities": ["detect"],
        "process_names": ["fixture"],
        "running_paths": ["/opt/etc/fixture"],
        "web_requires_package": "fixture-web",
        "web": {
            "scheme": "http",
            "port": 8080,
            "path": "/",
            "mode": "probe-required",
            "embed": True,
        },
        "compatibility": {
            "status": "requirements",
            "targets": ["aarch64-3.10"],
            "hints": ["Entware"],
        },
        "install": {
            "method": "structured",
            "packages": ["fixture"],
            "steps": [
                {"type": "opkg-update"},
                {"type": "opkg-install", "packages": ["fixture"], "ignore_failure": False},
            ],
        },
    }
    with tempfile.TemporaryDirectory() as tmp:
        builder.validate_manifest(valid, Path(tmp) / "contract-fixture.json")

    # Gate 4: representative invalid fixtures must fail closed.
    case = copy.deepcopy(valid)
    case["surprise_field"] = True
    expect_reject(builder, case, "contract-fixture.json", "unknown manifest keys")

    case = copy.deepcopy(valid)
    case["version_source"] = "magic"
    expect_reject(builder, case, "contract-fixture.json", "invalid version_source")

    case = copy.deepcopy(valid)
    case["compatibility"]["targets"] = ["mystery-cpu"]
    expect_reject(builder, case, "contract-fixture.json", "unsupported targets")

    case = copy.deepcopy(valid)
    case["install"]["steps"][0]["shell"] = "rm -rf /"
    expect_reject(builder, case, "contract-fixture.json", "unknown keys")

    case = copy.deepcopy(valid)
    case["web"]["mode"] = "external-only"
    case["web"]["embed"] = True
    expect_reject(builder, case, "contract-fixture.json", "web.embed requires")

    # Gate 5: runtime must recognize the same enums as schema/builder.
    props = schema["properties"]
    defs = schema["$defs"]

    version_sources = set(props["version_source"]["enum"])
    web_modes = set(props["web"]["properties"]["mode"]["enum"])
    methods = set(defs["plan"]["properties"]["method"]["enum"])
    steps = set(defs["step"]["properties"]["type"]["enum"])

    require_tokens(
        "version_source",
        function_body(registry_go, "validCatalogVersionSource"),
        version_sources,
    )
    require_tokens(
        "web.mode",
        function_body(registry_go, "validateCatalogWebMetadata"),
        web_modes,
    )
    require_tokens(
        "plan.method",
        function_body(registry_go, "validateCatalogPlan"),
        methods,
    )
    require_tokens(
        "step.type",
        function_body(registry_go, "validateCatalogPlan"),
        steps,
    )

    # Gate 6: runtime structs must expose the contract fields that previously drifted.
    runtime_tags = set(re.findall(r'json:"([^",]+)', runtime))
    required_runtime_tags = {
        "id", "kind", "name", "publisher", "project_url", "source",
        "managed", "builtin", "package_authoritative", "version_source",
        "conflicts", "capabilities", "process_names", "running_paths",
        "web_port", "web_port_source", "web_requires_package", "web",
        "detection", "compatibility", "presentation", "install", "update",
        "remove", "installer_url", "ignore_failure",
    }
    missing_tags = sorted(required_runtime_tags - runtime_tags)
    if missing_tags:
        fail("runtime JSON fields missing: " + ", ".join(missing_tags))

    print("MARKETPLACE_CONTRACT_GATE=PASS")
    print("SCHEMA_BUILDER_PARITY=PASS")
    print("CHECKED_IN_MANIFESTS=PASS")
    print("NEGATIVE_FIXTURES=PASS")
    print("RUNTIME_ENUM_PARITY=PASS")
    print("RUNTIME_FIELD_PARITY=PASS")

if __name__ == "__main__":
    main()
