#!/usr/bin/env python3
import importlib.util
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / "components" / "core" / "catalog.go"
REGISTRY_GO = ROOT / "components" / "core" / "routerforge_registry.go"
SUBMISSIONS = ROOT / "marketplace" / "submissions"
REGISTRY = ROOT / "marketplace" / "registry" / "index.json"
EMBEDDED = ROOT / "components" / "core" / "embedded" / "marketplace-index.json"
BUILDER = ROOT / "marketplace" / "build_registry.py"

def fail(message):
    raise SystemExit("legacy-fallback-removal: " + message)

def function_body(source, name):
    match = re.search(r"func\s+" + re.escape(name) + r"\s*\([^)]*\)[^{]*\{", source)
    if not match:
        fail("function missing: " + name)
    start = match.end()
    depth = 1
    i = start
    in_string = False
    escaped = False
    while i < len(source) and depth:
        ch = source[i]
        if in_string:
            if escaped:
                escaped = False
            elif ch == "\\":
                escaped = True
            elif ch == '"':
                in_string = False
        else:
            if ch == '"':
                in_string = True
            elif ch == "{":
                depth += 1
            elif ch == "}":
                depth -= 1
        i += 1
    if depth:
        fail("unbalanced function: " + name)
    return source[start:i - 1]

def first_string(body, field):
    match = re.search(r"\b" + re.escape(field) + r'\s*:\s*"([^"]*)"', body)
    return match.group(1) if match else ""

def int_field(body, field):
    match = re.search(r"\b" + re.escape(field) + r"\s*:\s*(\d+)", body)
    return int(match.group(1)) if match else 0

def string_slice_from_block(body, block_name, field):
    block = re.search(
        r"\b" + re.escape(block_name) + r"\s*:\s*catalog\w+\s*\{(.*?)\n\s*\},",
        body,
        re.S,
    )
    if not block:
        return []
    return string_slice(block.group(1), field)

def string_slice(body, field):
    match = re.search(
        r"\b" + re.escape(field) + r'\s*:\s*\[\]string\s*\{([^}]*)\}',
        body,
        re.S,
    )
    if not match:
        return []
    return re.findall(r'"([^"]*)"', match.group(1))

def install_block(body):
    match = re.search(r"\bInstall\s*:\s*catalogInstallPlan\s*\{(.*?)\n\s*\},", body, re.S)
    return match.group(1) if match else ""

def load_builder():
    spec = importlib.util.spec_from_file_location("routerforge_marketplace_builder", BUILDER)
    if spec is None or spec.loader is None:
        fail("cannot import marketplace builder")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

def normalize(value):
    return list(value or [])

def compare(label, legacy, manifest, item_id):
    if normalize(legacy) != normalize(manifest):
        fail(f"{item_id}: {label} drift legacy={legacy!r} manifest={manifest!r}")

def main():
    catalog = CATALOG.read_text(encoding="utf-8")
    registry_go = REGISTRY_GO.read_text(encoding="utf-8")
    builder = load_builder()

    # 1. Discover the legacy seed set from the actual Go catalog.
    catalog_body = function_body(catalog, "integrationCatalog")
    funcs = re.findall(r"\b([A-Za-z0-9_]+Integration)\(\)", catalog_body)
    if not funcs:
        fail("integrationCatalog has no integration seed functions")
    if len(funcs) != len(set(funcs)):
        fail("integrationCatalog contains duplicate seed functions")

    legacy = {}
    for fn in funcs:
        body = function_body(catalog, fn)
        item_id = first_string(body, "ID")
        if not item_id:
            fail(f"{fn}: ID missing")
        if item_id in legacy:
            fail(f"duplicate legacy ID: {item_id}")
        legacy[item_id] = (fn, body)

    # 2. Load all manifest/registry sources.
    manifests = {}
    for path in sorted(SUBMISSIONS.glob("*.json")):
        obj = json.loads(path.read_text(encoding="utf-8"))
        manifests[obj["id"]] = (path, obj)
        builder.validate_manifest(obj, path)

    public_doc = json.loads(REGISTRY.read_text(encoding="utf-8"))
    embedded_doc = json.loads(EMBEDDED.read_text(encoding="utf-8"))
    if public_doc != embedded_doc:
        fail("public and embedded registries differ")

    entries = {}
    for entry in public_doc.get("entries", []):
        item_id = entry.get("id")
        if item_id in entries:
            fail("duplicate registry ID: " + str(item_id))
        entries[item_id] = entry

    # 3. Every legacy integration must have an authoritative manifest and registry row.
    missing_manifest = sorted(set(legacy) - set(manifests))
    missing_registry = sorted(set(legacy) - set(entries))
    if missing_manifest:
        fail("legacy IDs without manifests: " + ", ".join(missing_manifest))
    if missing_registry:
        fail("legacy IDs without registry entries: " + ", ".join(missing_registry))

    # 4. Manifest digest/source must be exact and detection/web metadata must cover the legacy seed.
    for item_id in sorted(legacy):
        fn, body = legacy[item_id]
        path, manifest = manifests[item_id]
        entry = entries[item_id]

        expected_source = f"marketplace/submissions/{item_id}.json"
        digest = builder.digest_manifest(manifest)
        if entry.get("manifest_id") != item_id:
            fail(f"{item_id}: registry manifest_id mismatch")
        if entry.get("manifest_source") != expected_source:
            fail(f"{item_id}: manifest_source mismatch")
        if entry.get("manifest_sha256") != digest:
            fail(f"{item_id}: manifest digest mismatch")

        for go_field, json_field in (
            ("Name", "name"),
            ("Category", "category"),
            ("Description", "description"),
            ("ProjectURL", "project_url"),
            ("Source", "source"),
        ):
            legacy_value = first_string(body, go_field)
            if legacy_value and manifest.get(json_field, "") != legacy_value:
                fail(
                    f"{item_id}: {json_field} drift "
                    f"legacy={legacy_value!r} manifest={manifest.get(json_field)!r}"
                )

        detection = manifest.get("detection") or {}
        compare("detection.packages", string_slice_from_block(body, "Detection", "Packages"), detection.get("packages"), item_id)
        compare("detection.services", string_slice_from_block(body, "Detection", "Services"), detection.get("services"), item_id)
        compare("detection.paths", string_slice_from_block(body, "Detection", "Paths"), detection.get("paths"), item_id)
        compare("process_names", string_slice(body, "ProcessNames"), manifest.get("process_names"), item_id)

        legacy_port = int_field(body, "WebPort")
        if legacy_port and manifest.get("web_port", 0) != legacy_port:
            fail(f"{item_id}: web_port drift legacy={legacy_port} manifest={manifest.get('web_port')!r}")

        for go_field, json_field in (
            ("WebPortSource", "web_port_source"),
            ("WebRequiresPackage", "web_requires_package"),
        ):
            legacy_value = first_string(body, go_field)
            if legacy_value and manifest.get(json_field, "") != legacy_value:
                fail(
                    f"{item_id}: {json_field} drift "
                    f"legacy={legacy_value!r} manifest={manifest.get(json_field)!r}"
                )

        # Unverified migrations must never gain lifecycle authority while the fallback is removed.
        trust = (entry.get("trust") or {}).get("status", "")
        plan = manifest.get("install") or {}
        if trust == "unverified" and plan and plan.get("preview_only") is not True:
            fail(f"{item_id}: unverified manifest is not preview_only")

        # If the old seed was explicitly preview-only, an unverified replacement must stay so.
        old_plan = install_block(body)
        old_preview = bool(re.search(r"\bPreviewOnly\s*:\s*true", old_plan))
        if old_preview and trust == "unverified" and plan.get("preview_only") is not True:
            fail(f"{item_id}: fallback preview-only protection was lost")

    # 5. Prove the runtime has a manifest-only append path:
    #    when a seed target is absent, a non-builtin registry integration is appended.
    apply_body = function_body(registry_go, "applyRouterForgeRegistry")
    required_markers = (
        "if target := findCatalogItem(snapshot, incoming.ID, incoming.Kind); target != nil {",
        "if incoming.Builtin {",
        "snapshot.Integrations = append(snapshot.Integrations, incoming)",
    )
    for marker in required_markers:
        if marker not in apply_body:
            fail("manifest-only runtime append path changed: missing " + marker)

    # 6. The seed still exists in this gate phase. Removal is a separate commit.
    build_body = function_body(catalog, "buildCatalog")
    if "integrations := integrationCatalog()" not in build_body:
        fail("legacy fallback was already removed before the removal gate")

    print("LEGACY_FALLBACK_REMOVAL_GATE=PASS")
    print("LEGACY_SEED_COUNT=" + str(len(legacy)))
    print("MANIFEST_COVERAGE=PASS")
    print("REGISTRY_COVERAGE=PASS")
    print("DETECTION_METADATA_PARITY=PASS")
    print("WEB_METADATA_PARITY=PASS")
    print("UNVERIFIED_LIFECYCLE_NARROWING=PASS")
    print("MANIFEST_ONLY_RUNTIME_APPEND_PATH=PASS")
    print("LEGACY_FALLBACK_STILL_PRESENT=PASS")
    print("READY_FOR_FALLBACK_REMOVAL=YES")

if __name__ == "__main__":
    main()
